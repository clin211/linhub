package gin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

// 标准的 trace header 键
const (
	// W3C Trace Context 标准（最推荐）
	TraceParentHeaderKey = "traceparent"

	// 简单的 trace ID（使用最广泛）
	TraceIDHeaderKey = "X-Trace-Id"

	// 通用请求 ID（具有通用兼容性）
	RequestIDHeaderKey = "X-Request-Id"

	// 用于附加上下文的 tracestate
	TraceStateHeaderKey = "tracestate"

	// debug 模式下捕获 request/response body 的最大字节数，超过则截断。
	// 防止大文件上传/下载时的 OOM 风险。
	maxCapturedBodyBytes = 64 * 1024 // 64KB
)

// TraceInjectionMode 定义 trace 信息的注入方式
type TraceInjectionMode int

const (
	// InjectW3CTraceContext 注入完整的 W3C trace context（推荐）
	InjectW3CTraceContext TraceInjectionMode = iota
	// InjectTraceIDOnly 仅注入 trace ID
	InjectTraceIDOnly
	// InjectBoth 同时注入 W3C 格式和简单的 trace ID
	InjectBoth
	// InjectNone 禁用 trace 注入
	InjectNone
)

// ObservabilityOptions 持有 trace 注入的配置
type ObservabilityOptions struct {
	TraceInjectionMode TraceInjectionMode
	CustomTraceHeader  string   // 用于 trace ID 的自定义 header 名称
	SkipPaths          []string // 跳过日志记录的路径（支持通配符）
}

// Option 是用于配置中间件的函数式选项
type Option func(*ObservabilityOptions)

// WithTraceInjection 配置 trace 注入模式
func WithTraceInjection(mode TraceInjectionMode) Option {
	return func(o *ObservabilityOptions) {
		o.TraceInjectionMode = mode
	}
}

// WithCustomTraceHeader 为 trace ID 设置自定义 header 名称
func WithCustomTraceHeader(headerName string) Option {
	return func(o *ObservabilityOptions) {
		o.CustomTraceHeader = headerName
	}
}

// WithSkipPaths 配置要跳过的路径（支持精确匹配与通配符）
func WithSkipPaths(paths ...string) Option {
	return func(o *ObservabilityOptions) {
		o.SkipPaths = append(o.SkipPaths, paths...)
	}
}

// WithSkipMetrics 是一个便捷函数，用于跳过常见的指标端点
func WithSkipMetrics() Option {
	return func(o *ObservabilityOptions) {
		commonPaths := []string{
			"/health",
			"/healthz",
			"/health/*",
			"/ready",
			"/readiness",
			"/live",
			"/liveness",
			"/metrics",
			"/prometheus",
			"/status",
			"/ping",
			"/version",
			"/info",
			"/favicon.ico",
			"/robots.txt",
		}
		o.SkipPaths = append(o.SkipPaths, commonPaths...)
	}
}

// Observability 中间件，支持可配置的 trace 注入
func Observability(opts ...Option) gin.HandlerFunc {
	// 默认配置
	config := &ObservabilityOptions{
		TraceInjectionMode: InjectTraceIDOnly,
		SkipPaths:          []string{"/metrics"}, // 默认跳过 /metrics
	}

	// 应用选项
	for _, opt := range opts {
		opt(config)
	}

	return func(c *gin.Context) {
		start := time.Now()
		ctx := c.Request.Context()

		// 检查该请求是否需要跳过
		shouldSkip := shouldSkipPath(c.Request.URL.Path, c.Request.Method, config.SkipPaths)
		if shouldSkip {
			c.Next()
			return
		}

		// 提前提取 trace 信息
		span := trace.SpanFromContext(ctx)
		spanCtx := span.SpanContext()

		// 根据配置注入 trace headers（除非跳过 tracing）
		injectTraceHeaders(c, spanCtx, config)

		var requestBody string
		var responseBuffer bytes.Buffer

		// 只有在需要记录日志且开启了 debug 时才捕获 body
		isDebugLevel := isDebugEnabled()

		if isDebugLevel && c.Request.Body != nil {
			limitedReader := io.LimitReader(c.Request.Body, maxCapturedBodyBytes+1)
			bodyBytes, err := io.ReadAll(limitedReader)
			if err == nil {
				if len(bodyBytes) > maxCapturedBodyBytes {
					requestBody = string(bodyBytes[:maxCapturedBodyBytes]) + "...[truncated]"
				} else {
					requestBody = string(bodyBytes)
				}
				// 还原 body 供后续 handler 使用（包括截断的部分以及未读完的数据）
				c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewBuffer(bodyBytes), c.Request.Body))
			}
		}

		if isDebugLevel {
			writer := &bodyCaptureWriter{ResponseWriter: c.Writer, body: &responseBuffer, limit: maxCapturedBodyBytes}
			c.Writer = writer
		}

		c.Next()

		duration := time.Since(start).Seconds()

		// 构建结构化日志
		httpData := map[string]any{
			"request": map[string]any{
				"method": c.Request.Method,
				"path":   c.Request.URL.Path,
			},
			"response": map[string]any{
				"status_code": c.Writer.Status(),
			},
		}

		if isDebugLevel {
			httpData["request"].(map[string]any)["body"] = map[string]any{
				"content": requestBody,
				"bytes":   len(requestBody),
			}

			httpData["response"].(map[string]any)["body"] = map[string]any{
				"content": responseBuffer.String(),
				"bytes":   responseBuffer.Len(),
			}
		}

		logLevel := slog.LevelInfo
		if isDebugLevel {
			logLevel = slog.LevelDebug
		}

		slog.Log(ctx, logLevel, "HTTP request completed",
			"duration_sec", duration,
			"source", map[string]any{"ip": c.ClientIP()},
			"http", httpData,
			"user", map[string]any{"agent": c.Request.UserAgent()},
			"trace", map[string]any{"id": spanCtx.TraceID().String()},
			"span", map[string]any{"id": spanCtx.SpanID().String()},
		)
	}
}

// shouldSkipPath 根据配置检查是否应该跳过某个路径
func shouldSkipPath(path, method string, skipPaths []string) bool {
	for _, skipPath := range skipPaths {
		if matchPath(path, method, skipPath) {
			return true
		}
	}
	return false
}

// matchPath 将请求路径与跳过模式进行匹配
func matchPath(requestPath, method, pattern string) bool {
	// 处理特定方法的模式，例如 "GET /metrics"
	if strings.Contains(pattern, " ") {
		parts := strings.SplitN(pattern, " ", 2)
		if len(parts) == 2 {
			patternMethod := strings.ToUpper(strings.TrimSpace(parts[0]))
			patternPath := strings.TrimSpace(parts[1])

			if patternMethod != strings.ToUpper(method) {
				return false
			}
			return matchPathPattern(requestPath, patternPath)
		}
	}

	// 处理仅路径的模式
	return matchPathPattern(requestPath, pattern)
}

// matchPathPattern 根据模式匹配路径（支持通配符）
func matchPathPattern(path, pattern string) bool {
	// 精确匹配
	if path == pattern {
		return true
	}

	// 通配符支持
	if strings.Contains(pattern, "*") {
		return matchWildcard(path, pattern)
	}

	// 前缀匹配（如果 pattern 以 / 结尾）
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}

	return false
}

// matchWildcard 执行简单的通配符匹配
func matchWildcard(text, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// 简单的前缀/后缀通配符匹配
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		substr := pattern[1 : len(pattern)-1]
		return strings.Contains(text, substr)
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(text, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(text, prefix)
	}

	return text == pattern
}

// injectTraceHeaders 根据配置注入 trace headers
func injectTraceHeaders(c *gin.Context, spanCtx trace.SpanContext, config *ObservabilityOptions) {
	if !spanCtx.IsValid() {
		return
	}

	traceID := spanCtx.TraceID().String()
	spanID := spanCtx.SpanID().String()

	switch config.TraceInjectionMode {
	case InjectW3CTraceContext:
		// W3C Trace Context 格式：version-trace_id-parent_id-trace_flags
		traceFlags := "01" // 已采样
		if !spanCtx.IsSampled() {
			traceFlags = "00" // 未采样
		}
		traceparent := fmt.Sprintf("00-%s-%s-%s", traceID, spanID, traceFlags)
		c.Header(TraceParentHeaderKey, traceparent)

	case InjectTraceIDOnly:
		headerKey := TraceIDHeaderKey
		if config.CustomTraceHeader != "" {
			headerKey = config.CustomTraceHeader
		}
		c.Header(headerKey, traceID)

	case InjectBoth:
		// W3C 格式
		traceFlags := "01"
		if !spanCtx.IsSampled() {
			traceFlags = "00"
		}
		traceparent := fmt.Sprintf("00-%s-%s-%s", traceID, spanID, traceFlags)
		c.Header(TraceParentHeaderKey, traceparent)

		// 简单的 trace ID
		headerKey := TraceIDHeaderKey
		if config.CustomTraceHeader != "" {
			headerKey = config.CustomTraceHeader
		}
		c.Header(headerKey, traceID)

	case InjectNone:
		// 什么都不做
	}
}

// 常用配置的便捷函数

// ObservabilityWithW3CTraceContext 创建一个使用 W3C trace context 的中间件
func ObservabilityWithW3CTraceContext() gin.HandlerFunc {
	return Observability(WithTraceInjection(InjectW3CTraceContext))
}

// ObservabilityWithTraceID 创建一个使用简单 trace ID 的中间件
func ObservabilityWithTraceID() gin.HandlerFunc {
	return Observability(WithTraceInjection(InjectTraceIDOnly))
}

// ObservabilityWithCustomHeader 创建一个使用自定义 header 的中间件
func ObservabilityWithCustomHeader(headerName string) gin.HandlerFunc {
	return Observability(
		WithTraceInjection(InjectTraceIDOnly),
		WithCustomTraceHeader(headerName),
	)
}

// ObservabilitySkipMetrics 创建一个会跳过常见指标端点的中间件
func ObservabilitySkipMetrics() gin.HandlerFunc {
	return Observability(WithSkipMetrics())
}

// ObservabilityWithSkipPaths 创建一个使用自定义跳过路径的中间件
func ObservabilityWithSkipPaths(paths ...string) gin.HandlerFunc {
	return Observability(WithSkipPaths(paths...))
}

// bodyCaptureWriter 捕获并复制已写入的响应 body，但缓冲区不会超过 limit 字节，
// 防止大响应（如文件下载、流）导致 OOM。
type bodyCaptureWriter struct {
	gin.ResponseWriter
	body  *bytes.Buffer
	limit int
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	if remain := w.limit - w.body.Len(); remain > 0 {
		if len(b) <= remain {
			w.body.Write(b)
		} else {
			w.body.Write(b[:remain])
		}
	}
	return w.ResponseWriter.Write(b)
}

// isDebugEnabled 检查全局 logger 是否开启了 debug 级别日志
func isDebugEnabled() bool {
	return slog.Default().Enabled(context.Background(), slog.LevelDebug)
}
