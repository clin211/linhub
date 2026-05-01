package options

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

var _ IOptions = (*SlogOptions)(nil)

// SlogOptions 包含与 slog 相关的配置项。
type SlogOptions struct {
	// Level 指定要输出的最低日志级别。
	// 可选值：debug、info、warn、error
	Level string `json:"level,omitempty" mapstructure:"level"`
	// AddSource 在日志记录中添加源代码位置（file:line）
	AddSource bool `json:"add-source,omitempty" mapstructure:"add-source"`
	// Format 指定日志消息的结构。
	// 可选值：json、text
	Format string `json:"format,omitempty" mapstructure:"format"`
	// TimeFormat 指定 text 输出的时间格式。
	// 使用 Go 时间格式 layout。为空时表示使用 RFC3339。
	TimeFormat string `json:"time-format,omitempty" mapstructure:"time-format"`
	// Output 指定日志写入的位置。
	// 可选值：stdout、stderr 或文件路径
	Output string `json:"output,omitempty" mapstructure:"output"`
}

// NewSlogOptions 创建一个使用默认参数的 Options 对象。
func NewSlogOptions() *SlogOptions {
	return &SlogOptions{
		Level:      "info",
		AddSource:  false,
		Format:     "text",
		TimeFormat: "",
		Output:     "stdout",
	}
}

// Validate 校验传递给 SlogOptions 的命令行标志。
func (o *SlogOptions) Validate() []error {
	var errs []error

	// 校验日志级别
	switch strings.ToUpper(strings.TrimSpace(o.Level)) {
	case "DEBUG", "INFO", "WARN", "WARNING", "ERROR":
	default:
		errs = append(errs, fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", o.Level))
	}

	// 校验日志格式
	switch o.Format {
	case "json", "text":
	default:
		errs = append(errs, fmt.Errorf("invalid log format: %s (must be json or text)", o.Format))
	}

	// 校验输出位置
	if o.Output != "stdout" && o.Output != "stderr" && o.Output != "" {
		// 检查是否为有效的文件路径（基础校验）
		if !filepath.IsAbs(o.Output) && !strings.Contains(o.Output, "/") {
			errs = append(errs, fmt.Errorf("invalid output path: %s", o.Output))
		}
	}

	return errs
}

// AddFlags 为该配置添加命令行标志。
func (o *SlogOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Level, fullPrefix+".level", o.Level, "设置日志级别。允许的级别：debug、info、warn、error。")
	fs.StringVar(&o.Format, fullPrefix+".format", o.Format, "设置日志格式。允许的格式：json、text。")
	fs.BoolVar(&o.AddSource, fullPrefix+".add-source", o.AddSource, "在日志记录中添加源文件 file:line 信息。")
	fs.StringVar(&o.TimeFormat, fullPrefix+".time-format", o.TimeFormat, ""+
		"text 日志使用 Go 时间 layout 格式的时间格式。留空表示使用 RFC3339。"+
		"示例：'2006-01-02 15:04:05'")
	fs.StringVar(&o.Output, fullPrefix+".output", o.Output, "日志输出目的地（stdout、stderr 或文件路径）。")
}

// ToSlogLevel 将字符串形式的日志级别转换为 slog.Level。
func (o *SlogOptions) ToSlogLevel() slog.Level {
	switch strings.ToUpper(strings.TrimSpace(o.Level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		// 未知级别时默认为 INFO
		return slog.LevelInfo
	}
}

// GetWriter 根据输出配置返回相应的 io.Writer。
func (o *SlogOptions) GetWriter() (io.Writer, error) {
	switch o.Output {
	case "stdout", "":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		// 文件输出
		file, err := os.OpenFile(o.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", o.Output, err)
		}
		return file, nil
	}
}

// BuildHandler 根据配置创建一个 slog.Handler。
func (o *SlogOptions) BuildHandler() (slog.Handler, error) {
	writer, err := o.GetWriter()
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:     o.ToSlogLevel(),
		AddSource: o.AddSource,
	}

	// 为 text handler 设置自定义时间格式
	if o.Format == "text" && o.TimeFormat != "" {
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format(o.TimeFormat))
			}
			return a
		}
	}

	var handler slog.Handler
	switch o.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		handler = slog.NewTextHandler(writer, opts)
	}

	return handler, nil
}

// BuildLogger 构建并返回一个已配置好的 slog.Logger 实例，不会影响全局 logger。
func (o *SlogOptions) BuildLogger() (*slog.Logger, error) {
	handler, err := o.BuildHandler()
	if err != nil {
		return nil, err
	}
	return slog.New(handler), nil
}

// Apply 将配置应用到全局默认的 slog logger 上。
func (o *SlogOptions) Apply() error {
	logger, err := o.BuildLogger()
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	return nil
}
