package log

import (
	"context"
	"log/slog"
)

// fanoutHandler 把日志记录同时分发给多个底层 handler。
// 用于本地 stdout/file 与 OTel bridge 同时输出的场景。
//
// 注意：任意一个底层 handler 出错都不会终止其他 handler 的处理；
// 我们记录错误但不向上传播，避免日志失败导致业务路径报错。
type fanoutHandler struct {
	handlers []slog.Handler
}

// Enabled 任意一个底层 handler 启用即视为启用。
func (f fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle 把记录派发给所有底层 handler。
func (f fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range f.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		// 每个 handler 拿到一份独立的 record clone，避免共享 attributes
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// WithAttrs 派发到每个底层 handler。
func (f fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return fanoutHandler{handlers: out}
}

// WithGroup 派发到每个底层 handler。
func (f fanoutHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		out[i] = h.WithGroup(name)
	}
	return fanoutHandler{handlers: out}
}
