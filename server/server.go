package server

import (
	"context"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

// Server 定义所有服务器类型的接口.
type Server interface {
	// RunOrDie 运行服务器，如果运行失败会退出程序（OrDie的含义所在）.
	RunOrDie()
	// GracefulStop 方法用来优雅关停服务器。关停服务器时需要处理 context 的超时时间.
	GracefulStop(ctx context.Context)
}

// Serve 启动服务器，并阻塞直到上下文被取消.
// 它确保在上下文结束时服务器能被优雅地关闭.
func Serve(ctx context.Context, srv Server) error {
	go srv.RunOrDie()

	// 阻塞直到上下文被取消或终止.
	<-ctx.Done()

	// 优雅地关闭服务器.
	klog.InfoS("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 优雅地停止服务器.
	srv.GracefulStop(ctx)

	klog.InfoS("Server exited successfully.")

	return nil
}

// protocolName 从 http.Server 中获取协议名.
func protocolName(server *http.Server) string {
	if server.TLSConfig != nil {
		return "https"
	}
	return "http"
}
