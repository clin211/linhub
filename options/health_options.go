package options

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/spf13/pflag"
)

var _ IOptions = (*HealthOptions)(nil)

// HealthOptions 定义健康检查相关的选项。
type HealthOptions struct {
	// 通过暴露 profiling 信息来启用调试。
	HTTPProfile        bool   `json:"enable-http-profiler" mapstructure:"enable-http-profiler"`
	HealthCheckPath    string `json:"check-path" mapstructure:"check-path"`
	HealthCheckAddress string `json:"check-address" mapstructure:"check-address"`
}

// NewHealthOptions 创建一个 `zero` 值实例。
func NewHealthOptions() *HealthOptions {
	return &HealthOptions{
		HTTPProfile:        false,
		HealthCheckPath:    "/healthz",
		HealthCheckAddress: "0.0.0.0:20250",
	}
}

// Validate 校验传递给 HealthOptions 的命令行标志。
func (o *HealthOptions) Validate() []error {
	errs := []error{}

	return errs
}

// AddFlags 将与特定 APIServer 的健康检查相关的命令行标志添加到指定的 FlagSet。
func (o *HealthOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.BoolVar(&o.HTTPProfile, fullPrefix+".enable-http-profiler", o.HTTPProfile, "通过 HTTP 暴露运行时 profiling 数据。")
	fs.StringVar(&o.HealthCheckPath, fullPrefix+".check-path", o.HealthCheckPath, "指定存活健康检查的请求路径。")
	fs.StringVar(&o.HealthCheckAddress, fullPrefix+".check-address", o.HealthCheckAddress, "指定存活健康检查的绑定地址。")
}

// ServeHealthCheck 启动健康检查服务。
// 监听失败时返回 error 由调用方决定退出策略，不再直接 os.Exit；
// 服务器正常关闭（http.ErrServerClosed）会被忽略。
func (o *HealthOptions) ServeHealthCheck() error {
	r := mux.NewRouter()

	r.HandleFunc(o.HealthCheckPath, handler).Methods(http.MethodGet)
	if o.HTTPProfile {
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/{_:.*}", pprof.Index)
	}

	slog.Info("Starting health check server", "path", o.HealthCheckPath, "addr", o.HealthCheckAddress)
	if err := http.ListenAndServe(o.HealthCheckAddress, r); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Error serving health check endpoint", "error", err)
		return err
	}
	return nil
}

func handler(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(`{"status": "ok"}`))
}
