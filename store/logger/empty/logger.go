package empty

import "context"

// emptyLogger 是一个空操作的日志器，实现了 Logger 接口。
// 它不执行任何日志操作。
type emptyLogger struct{}

// NewLogger 创建并返回一个新的 emptyLogger 实例。
func NewLogger() *emptyLogger {
	return &emptyLogger{} // 返回一个新的 emptyLogger 实例
}

// Error 是一个空操作的方法，满足 Logger 接口。
// 它不记录任何错误消息或上下文。
func (l *emptyLogger) Error(ctx context.Context, err error, msg string, kvs ...any) {
	// 不执行任何错误日志记录操作
}
