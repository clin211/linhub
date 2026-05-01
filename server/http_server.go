package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	genericoptions "github.com/clin211/linhub/options"
)

// 默认 HTTP server 超时配置（生产环境必须设置以防 Slowloris 等慢速攻击）
const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
)

// HTTPServer 代表一个 HTTP 服务器.
type HTTPServer struct {
	srv *http.Server
}

// NewHTTPServer 创建一个新的 HTTP 服务器实例.
// 若 httpOptions.Timeout > 0，则同步应用到 ReadTimeout/WriteTimeout，否则使用安全默认值。
func NewHTTPServer(httpOptions *genericoptions.HTTPOptions, tlsOptions *genericoptions.TLSOptions, handler http.Handler) *HTTPServer {
	var tlsConfig *tls.Config
	if tlsOptions != nil && tlsOptions.UseTLS {
		tlsConfig = tlsOptions.MustTLSConfig()
	}

	readTimeout := defaultReadTimeout
	writeTimeout := defaultWriteTimeout
	if httpOptions != nil && httpOptions.Timeout > 0 {
		readTimeout = httpOptions.Timeout
		writeTimeout = httpOptions.Timeout
	}

	return &HTTPServer{
		srv: &http.Server{
			Addr:              httpOptions.Addr,
			Handler:           handler,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: defaultReadHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       defaultIdleTimeout,
		},
	}
}

// RunOrDie 启动 HTTP 服务器并在出错时记录致命错误.
func (s *HTTPServer) RunOrDie() {
	klog.InfoS("Start to listening the incoming requests", "protocol", protocolName(s.srv), "addr", s.srv.Addr)
	// 默认启动 HTTP 服务器
	serveFn := func() error { return s.srv.ListenAndServe() }
	if s.srv.TLSConfig != nil {
		serveFn = func() error { return s.srv.ListenAndServeTLS("", "") }
	}

	if err := serveFn(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		klog.Fatalf("Failed to server HTTP(s) server: %v", err)
	}
}

// GracefulStop 优雅地关闭 HTTP 服务器.
func (s *HTTPServer) GracefulStop(ctx context.Context) {
	klog.InfoS("Gracefully stop HTTP(s) server")
	if err := s.srv.Shutdown(ctx); err != nil {
		klog.ErrorS(err, "HTTP(s) server forced to shutdown")
	}
}
