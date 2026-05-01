package onex

import (
	"context"

	"github.com/clin211/linhub/log"
)

// onexLogger 是 store.Logger 的一个实现，桥接到 linhub/log 包。
//
// 设计说明：
//   - 签名严格遵循 store.Logger.Error(ctx, err, msg, kvs...)；
//   - 当前 ctx 未被直接消费——linhub/log 的 Errorw 不接收 ctx，
//     如需把 trace_id / request_id 等 ctx 字段输出到日志，请通过
//     log.WithContextExtractor 在全局 logger 上配置提取器；
//   - 历史上本文件的 Error 缺少 ctx 形参，导致它并未真正实现
//     store.Logger 接口（无人引用因此未暴露）。本次修复对齐签名。
type onexLogger struct{}

// NewLogger 创建并返回一个新的 onexLogger 实例。
func NewLogger() *onexLogger {
	return &onexLogger{}
}

// Error 使用 linhub/log 包记录错误消息。ctx 当前不直接使用，详见类型注释。
func (l *onexLogger) Error(_ context.Context, err error, msg string, kvs ...any) {
	log.Errorw(err, msg, kvs...)
}
