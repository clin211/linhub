package log

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// Format 表示日志的编码格式。
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Options 包含日志的配置选项（基于 slog 标准库）。
type Options struct {
	// Level 指定最低日志级别：debug、info、warn、error。
	Level string `json:"level,omitempty" mapstructure:"level"`
	// Format 指定日志输出格式：text 或 json。
	Format string `json:"format,omitempty" mapstructure:"format"`
	// AddSource 控制是否在日志记录中携带源文件位置（file:line）。
	AddSource bool `json:"add-source,omitempty" mapstructure:"add-source"`
	// OutputPaths 指定日志输出位置。支持 stdout、stderr 或绝对/相对文件路径。
	OutputPaths []string `json:"output-paths,omitempty" mapstructure:"output-paths"`
	// EnableOTel 控制是否启用 OpenTelemetry log bridge（otelslog）。
	// 启用后日志会同时被发送到 OTel 全局 LoggerProvider，便于在分布式系统中聚合。
	EnableOTel bool `json:"enable-otel,omitempty" mapstructure:"enable-otel"`
	// ServiceName 在启用 OTel 时作为 logger 的 instrumentation name。
	ServiceName string `json:"service-name,omitempty" mapstructure:"service-name"`

	// 以下字段保留是为了与历史 API 兼容，但在基于 slog 的实现中不再生效。
	// Deprecated: slog 不区分这些细节，仅保留兼容字段名。
	DisableCaller     bool `json:"disable-caller,omitempty" mapstructure:"disable-caller"`
	DisableStacktrace bool `json:"disable-stacktrace,omitempty" mapstructure:"disable-stacktrace"`
	EnableColor       bool `json:"enable-color,omitempty" mapstructure:"enable-color"`
}

// NewOptions 使用合理的默认值创建一个 Options 实例。
func NewOptions() *Options {
	return &Options{
		Level:       slog.LevelInfo.String(),
		Format:      string(FormatText),
		AddSource:   false,
		OutputPaths: []string{"stdout"},
		EnableOTel:  false,
		ServiceName: "linhub",
	}
}

// Validate 校验 Options 的字段。
func (o *Options) Validate() []error {
	var errs []error

	if _, err := parseLevel(o.Level); err != nil {
		errs = append(errs, err)
	}

	switch Format(o.Format) {
	case FormatText, FormatJSON, "":
	default:
		errs = append(errs, fmt.Errorf("invalid log format %q (allowed: text, json)", o.Format))
	}

	return errs
}

// AddFlags 为 Options 注册命令行参数。
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Level, "log.level", o.Level, "Minimum log output `LEVEL` (debug/info/warn/error).")
	fs.StringVar(&o.Format, "log.format", o.Format, "Log output `FORMAT` (text or json).")
	fs.BoolVar(&o.AddSource, "log.add-source", o.AddSource, "Add source file:line to log records.")
	fs.StringSliceVar(&o.OutputPaths, "log.output-paths", o.OutputPaths, "Output paths (stdout/stderr/file path).")
	fs.BoolVar(&o.EnableOTel, "log.enable-otel", o.EnableOTel, "Enable OpenTelemetry log bridge (otelslog).")
	fs.StringVar(&o.ServiceName, "log.service-name", o.ServiceName, "Service name used as OTel logger instrumentation name.")

	// 兼容字段（不再生效，仅保留以避免破坏既有配置）
	fs.BoolVar(&o.DisableCaller, "log.disable-caller", o.DisableCaller, "Deprecated: superseded by --log.add-source")
	fs.BoolVar(&o.DisableStacktrace, "log.disable-stacktrace", o.DisableStacktrace, "Deprecated: slog does not auto-emit stacktrace")
	fs.BoolVar(&o.EnableColor, "log.enable-color", o.EnableColor, "Deprecated: not honored by slog backend")
}

// resolveWriter 将一组 outputPaths 合并为单一 io.Writer。
// 支持 stdout/stderr 以及文件路径，文件会以追加模式打开。
func (o *Options) resolveWriter() (io.Writer, error) {
	paths := o.OutputPaths
	if len(paths) == 0 {
		return os.Stdout, nil
	}

	writers := make([]io.Writer, 0, len(paths))
	for _, p := range paths {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "", "stdout":
			writers = append(writers, os.Stdout)
		case "stderr":
			writers = append(writers, os.Stderr)
		default:
			f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return nil, fmt.Errorf("log: open output %q: %w", p, err)
			}
			writers = append(writers, f)
		}
	}
	if len(writers) == 1 {
		return writers[0], nil
	}
	return io.MultiWriter(writers...), nil
}

// parseLevel 把字符串形式的级别解析为 slog.Level。
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "panic", "fatal":
		return slog.LevelError, nil // slog 没有 panic/fatal 级，对齐到 error
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q", s)
	}
}
