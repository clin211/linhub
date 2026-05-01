package store

import (
	"context"
)

// Logger 定义一个接口，用于记录带上下文信息的错误日志。
type Logger interface {
	// Error 记录带关联上下文的错误消息。
	Error(ctx context.Context, err error, message string, kvs ...any)
}
