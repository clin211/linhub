package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryOptions 控制 gRPC panic recovery 行为。
type RecoveryOptions struct {
	OnPanic func(ctx context.Context, recovered any, stack []byte)
}

// RecoveryOption 是 Recovery 中间件的可选配置。
type RecoveryOption func(*RecoveryOptions)

// WithRecoveryHook 注入 panic 钩子（如上报告警）。
func WithRecoveryHook(fn func(ctx context.Context, recovered any, stack []byte)) RecoveryOption {
	return func(o *RecoveryOptions) { o.OnPanic = fn }
}

// Recovery 是带结构化日志和可选钩子的 gRPC panic recovery 拦截器。
// panic 不会暴露给客户端，仅返回 codes.Internal。
func Recovery(opts ...RecoveryOption) grpc.UnaryServerInterceptor {
	cfg := &RecoveryOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()

				slog.ErrorContext(ctx, "panic recovered in grpc handler",
					"recovered", fmt.Sprintf("%v", rec),
					"stack", string(stack),
					"method", info.FullMethod,
				)

				if cfg.OnPanic != nil {
					func() {
						defer func() { _ = recover() }()
						cfg.OnPanic(ctx, rec, stack)
					}()
				}

				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}
