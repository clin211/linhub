package resty

import (
	"fmt"
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

// Errorf 输出 Error 级日志
func (l *Logger) Errorf(format string, v ...any) {
	l.logger.Error(fmt.Sprintf(format, v...))
}

// Warnf 输出 Warn 级日志
func (l *Logger) Warnf(format string, v ...any) {}

// Debugf 输出 Debug 级日志
func (l *Logger) Debugf(format string, v ...any) {
	l.logger.Debug(fmt.Sprintf(format, v...))
}
