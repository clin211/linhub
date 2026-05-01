package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// --- Trace Header 常量 ---
const (
	TraceParentHeaderKey = "traceparent"
	TraceIDHeaderKey     = "X-Trace-Id"
	RequestIDHeaderKey   = "X-Request-Id"
	TraceStateHeaderKey  = "tracestate"
)

// --- Trace 注入模式 ---
type TraceInjectionMode int

const (
	InjectW3CTraceContext TraceInjectionMode = iota
	InjectTraceIDOnly
	InjectBoth
	InjectNone
)

// --- 配置 ---
type ObservabilityOptions struct {
	TraceInjectionMode TraceInjectionMode
	CustomTraceHeader  string
}

// --- Option 模式 ---
type Option func(*ObservabilityOptions)

func WithTraceInjection(mode TraceInjectionMode) Option {
	return func(o *ObservabilityOptions) { o.TraceInjectionMode = mode }
}

func WithCustomTraceHeader(header string) Option {
	return func(o *ObservabilityOptions) { o.CustomTraceHeader = header }
}

// --- 主拦截器 ---
func Observability(opts ...Option) grpc.UnaryServerInterceptor {
	cfg := &ObservabilityOptions{TraceInjectionMode: InjectTraceIDOnly}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		spanCtx := trace.SpanFromContext(ctx).SpanContext()

		// 仅在响应中注入 trace 相关 metadata，避免把客户端发来的所有 header 反弹回去
		// （包括 Authorization、cookie 等敏感信息）。
		injectTraceResponseHeaders(ctx, spanCtx, cfg)
		isDebugLevel := isDebugEnabled()

		// 可选：捕获请求负载
		var requestBody string
		if isDebugLevel && req != nil {
			data, _ := json.Marshal(req)
			requestBody = string(data)
		}

		// 处理 RPC 调用
		resp, err := handler(ctx, req)

		// 可选：捕获响应负载
		var responseBody string
		if isDebugLevel && resp != nil {
			data, _ := json.Marshal(resp)
			responseBody = string(data)
		}

		duration := time.Since(start).Seconds()

		// Peer 信息
		var clientIP string
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		status := "OK"
		if err != nil {
			status = "ERROR"
		}

		// 构建结构化日志
		rpcData := map[string]any{
			"request": map[string]any{
				"method": info.FullMethod,
			},
			"response": map[string]any{
				"status": status,
			},
		}

		if isDebugLevel {
			rpcData["request"] = map[string]any{
				"body": map[string]any{
					"content": requestBody,
					"bytes":   len(requestBody),
				},
			}
			rpcData["response"] = map[string]any{
				"body": map[string]any{
					"content": responseBody,
					"bytes":   len(responseBody),
				},
			}
		}

		logLevel := slog.LevelInfo
		if isDebugLevel {
			logLevel = slog.LevelDebug
		}

		slog.Log(ctx, logLevel, "gRPC request completed",
			"duration_sec", duration,
			"source", map[string]any{"ip": clientIP},
			"rpc", rpcData,
			"trace", map[string]any{"id": spanCtx.TraceID().String()},
			"span", map[string]any{"id": spanCtx.SpanID().String()},
		)

		return resp, err
	}
}

// injectTraceResponseHeaders 仅把 trace 相关的 metadata 写到响应 header，
// 避免回弹整个 incoming metadata 导致敏感信息泄露。
func injectTraceResponseHeaders(ctx context.Context, spanCtx trace.SpanContext, cfg *ObservabilityOptions) {
	if !spanCtx.IsValid() {
		return
	}

	traceID := spanCtx.TraceID().String()
	spanID := spanCtx.SpanID().String()

	out := metadata.MD{}

	switch cfg.TraceInjectionMode {
	case InjectW3CTraceContext:
		traceFlags := "01"
		if !spanCtx.IsSampled() {
			traceFlags = "00"
		}
		out.Set(TraceParentHeaderKey, fmt.Sprintf("00-%s-%s-%s", traceID, spanID, traceFlags))

	case InjectTraceIDOnly:
		header := TraceIDHeaderKey
		if cfg.CustomTraceHeader != "" {
			header = cfg.CustomTraceHeader
		}
		out.Set(header, traceID)

	case InjectBoth:
		traceFlags := "01"
		if !spanCtx.IsSampled() {
			traceFlags = "00"
		}
		out.Set(TraceParentHeaderKey, fmt.Sprintf("00-%s-%s-%s", traceID, spanID, traceFlags))
		header := TraceIDHeaderKey
		if cfg.CustomTraceHeader != "" {
			header = cfg.CustomTraceHeader
		}
		out.Set(header, traceID)

	case InjectNone:
		return
	}

	if len(out) == 0 {
		return
	}
	_ = grpc.SetHeader(ctx, out)
}

func isDebugEnabled() bool {
	return slog.Default().Enabled(context.Background(), slog.LevelDebug)
}

// --- 便捷包装函数 ---
func ObservabilityWithW3CTraceContext() grpc.UnaryServerInterceptor {
	return Observability(WithTraceInjection(InjectW3CTraceContext))
}

func ObservabilityWithTraceID() grpc.UnaryServerInterceptor {
	return Observability(WithTraceInjection(InjectTraceIDOnly))
}

func ObservabilityWithCustomHeader(headerName string) grpc.UnaryServerInterceptor {
	return Observability(
		WithTraceInjection(InjectTraceIDOnly),
		WithCustomTraceHeader(headerName),
	)
}
