package onex

import (
	"github.com/clin211/linhub/log"
	"github.com/clin211/linhub/logger"
)

// onexLogger 提供了 logger.Logger 接口的一个实现。
type onexLogger struct{}

// 确保 onexLogger 实现了 logger.Logger 接口。
var _ logger.Logger = (*onexLogger)(nil)

// NewLogger 创建一个新的 onexLogger 实例。
func NewLogger() *onexLogger {
	return &onexLogger{}
}

// Debug 记录一条调试消息以及任意附加的键值对。
func (l *onexLogger) Debug(msg string, kvs ...any) {
	log.Debugw(msg, kvs...)
}

// Warn 记录一条警告消息以及任意附加的键值对。
func (l *onexLogger) Warn(msg string, kvs ...any) {
	log.Warnw(msg, kvs...)
}

// Info 记录一条一般信息消息以及任意附加的键值对。
func (l *onexLogger) Info(msg string, kvs ...any) {
	log.Infow(msg, kvs...)
}

// Error 记录一条错误消息以及任意附加的键值对。
func (l *onexLogger) Error(msg string, kvs ...any) {
	log.Errorw(nil, msg, kvs...)
}
