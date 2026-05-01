package breeze

import (
	"context"
	"log/slog"
)

// Logger 是基于 log/slog 实现的 Breeze 日志器。
type Logger struct {
	logger *slog.Logger
}

// NewLogger 使用默认的 slog 实例创建一个新的日志器。
func NewLogger() *Logger {
	return &Logger{logger: slog.Default()}
}

// Debug 输出调试信息。
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.logger.DebugContext(ctx, msg, args...)
}

// Info 输出一般信息。
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.logger.InfoContext(ctx, msg, args...)
}

// Warn 输出警告信息。
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.logger.WarnContext(ctx, msg, args...)
}

// Error 输出错误信息。
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.logger.ErrorContext(ctx, msg, args...)
}
