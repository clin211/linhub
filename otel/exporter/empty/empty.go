package tracing

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Exporter 实现了 sdktrace.SpanExporter 接口.
type Exporter struct{}

// 确保 Exporter 实现了 sdktrace.SpanExporter 接口.
var _ sdktrace.SpanExporter = (*Exporter)(nil)

// ExportSpans 使用 slog 记录已完成的 Span.
func (e *Exporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

// Shutdown 在需要时关闭日志器（此处为空操作）.
func (e *Exporter) Shutdown(ctx context.Context) error {
	return nil
}

// NewEmptyExporter 创建并返回一个 Exporter 的新实例，该实例满足
// OpenTelemetry sdktrace.SpanExporter 接口，但不会输出、存储
// 或转发任何 Span 数据.
func NewEmptyExporter() *Exporter {
	return &Exporter{}
}
