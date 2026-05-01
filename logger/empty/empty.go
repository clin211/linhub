package empty

import "github.com/clin211/linhub/logger"

// emptyLogger 是 logger.Logger 接口的一个空操作实现。
// 在需要日志器但又不希望产生日志输出的场景中非常有用。
type emptyLogger struct{}

// 确保 emptyLogger 实现了 logger.Logger 接口。
var _ logger.Logger = (*emptyLogger)(nil)

// NewLogger 返回一个新的空日志器实例。
func NewLogger() *emptyLogger {
	return &emptyLogger{}
}

// Debug 在 Debug 级别记录消息。该实现不执行任何操作。
func (l *emptyLogger) Debug(msg string, keysAndValues ...any) {}

// Warn 在 Warn 级别记录消息。该实现不执行任何操作。
func (l *emptyLogger) Warn(msg string, keysAndValues ...any) {}

// Info 在 Info 级别记录消息。该实现不执行任何操作。
func (l *emptyLogger) Info(msg string, keysAndValues ...any) {}

// Error 在 Error 级别记录消息。该实现不执行任何操作。
func (l *emptyLogger) Error(msg string, keysAndValues ...any) {}
