package options

import (
	"context"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

var _ IOptions = (*JaegerOptions)(nil)

// JaegerOptions 定义 Jaeger 客户端的选项。
type JaegerOptions struct {
	// Server 是 Jaeger 服务器的 URL
	Server      string `json:"server,omitempty" mapstructure:"server"`
	ServiceName string `json:"service-name,omitempty" mapstructure:"service-name"`
	Env         string `json:"env,omitempty" mapstructure:"env"`
}

// NewJaegerOptions 创建一个 `zero` 值实例。
func NewJaegerOptions() *JaegerOptions {
	return &JaegerOptions{
		Server: "http://127.0.0.1:14268/api/traces",
		Env:    "dev",
	}
}

// Validate 校验传递给 JaegerOptions 的命令行标志。
func (o *JaegerOptions) Validate() []error {
	errs := []error{}

	return errs
}

// AddFlags 将与特定 APIServer 的 Jaeger 相关的命令行标志添加到指定的 FlagSet。
func (o *JaegerOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Server, fullPrefix+".server", o.Server, ""+
		"Server 是 Jaeger 服务器的 URL。")
	fs.StringVar(&o.ServiceName, fullPrefix+".service-name", o.ServiceName, ""+
		"指定 Jaeger 资源的服务名称。")
	fs.StringVar(&o.Env, fullPrefix+".env", o.Env, "指定部署环境（dev/test/staging/prod）。")
}

func (o *JaegerOptions) SetTracerProvider() error {
	// 创建 Jaeger 导出器
	opts := make([]otlptracegrpc.Option, 0)
	opts = append(opts, otlptracegrpc.WithEndpoint(o.Server), otlptracegrpc.WithInsecure())
	exporter, err := otlptracegrpc.New(context.Background(), opts...)
	if err != nil {
		return err
	}

	res, err := resource.New(context.Background(), resource.WithAttributes(
		semconv.ServiceNameKey.String(o.ServiceName),
		attribute.String("env", o.Env),
		attribute.String("exporter", "jaeger"),
	))
	if err != nil {
		return err
	}

	// 批量 span 处理器，用于在导出前聚合 span。
	bsp := tracesdk.NewBatchSpanProcessor(exporter)
	tp := tracesdk.NewTracerProvider(
		// 基于父 span 将采样率设置为 100%
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(1.0))),
		// 在生产环境中务必使用批量处理。
		tracesdk.WithSpanProcessor(bsp),
		// 在 Resource 中记录该应用的相关信息。
		tracesdk.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return nil
}
