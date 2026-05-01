package logger

// Logger 定义了在不同级别记录日志的方法。
type Logger interface {
	// Debug 在 debug 级别记录一条消息，可附带可选的键值对。
	Debug(message string, keysAndValues ...any)

	// Warn 在 warning 级别记录一条消息，可附带可选的键值对。
	Warn(message string, keysAndValues ...any)

	// Info 在 info 级别记录一条消息，可附带可选的键值对。
	Info(message string, keysAndValues ...any)

	// Error 在 error 级别记录一条消息，可附带可选的键值对。
	Error(message string, keysAndValues ...any)
}
