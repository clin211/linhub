package gin

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/clin211/linhub/errx"
)

// RecoveryOptions 控制 panic recovery 行为。
type RecoveryOptions struct {
	// IncludeStack 控制是否在响应中携带 stack（生产环境应关闭，仅日志中保留）
	IncludeStack bool
	// OnPanic 在 recover 后被调用，可用于上报到 Sentry/告警平台。
	OnPanic func(c *gin.Context, recovered any, stack []byte)
}

// RecoveryOption 是 Recovery 中间件的可选配置。
type RecoveryOption func(*RecoveryOptions)

// WithIncludeStack 控制响应中是否携带 stack。
func WithIncludeStack(b bool) RecoveryOption {
	return func(o *RecoveryOptions) { o.IncludeStack = b }
}

// WithOnPanic 注入 panic 钩子（如上报告警）。
func WithOnPanic(fn func(c *gin.Context, recovered any, stack []byte)) RecoveryOption {
	return func(o *RecoveryOptions) { o.OnPanic = fn }
}

// Recovery 是带结构化日志和可选钩子的 panic recovery 中间件。
// 默认在响应中只暴露通用错误信息，stack 仅写入日志，避免敏感信息泄露。
func Recovery(opts ...RecoveryOption) gin.HandlerFunc {
	cfg := &RecoveryOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()

				slog.ErrorContext(c.Request.Context(), "panic recovered",
					"recovered", fmt.Sprintf("%v", rec),
					"stack", string(stack),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"client_ip", c.ClientIP(),
				)

				if cfg.OnPanic != nil {
					func() {
						defer func() { _ = recover() }() // hook 自身 panic 不能传播
						cfg.OnPanic(c, rec, stack)
					}()
				}

				bizErr := errx.NewBizError(errx.CodeInternalServer, "Internal", "internal server error")
				if cfg.IncludeStack {
					bizErr = bizErr.WithDetails(string(stack))
				}
				resp := errx.FromBizError(bizErr)

				if !c.Writer.Written() {
					// panic 属于系统级故障，仍然返回 500（与 errx.GetHTTPCode(CodeInternalServer) 一致）
					c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
				} else {
					c.Abort()
				}
			}
		}()

		c.Next()
	}
}
