package options

import (
	"time"

	"github.com/spf13/pflag"
	logsapi "k8s.io/component-base/logs/api/v1"
)

var _ IOptions = (*LogOptions)(nil)

// LogOptions 包含与日志相关的配置项。
type LogOptions struct {
	// Format 标志指定日志消息的结构。
	// format 的默认值是 `text`
	Format string `json:"format,omitempty" mapstructure:"format"`
	// 日志刷新之间的最大纳秒数（即 1s = 1000000000）。
	// 如果所选日志后端在写入日志消息时不进行缓冲，则忽略此项。
	FlushFrequency time.Duration `json:"flush-frequency" mapstructure:"flush-frequency"`
	// Verbosity 是决定哪些日志消息会被记录的阈值。
	// 默认值为零，仅记录最重要的消息。值越大，记录的消息越多。
	// 错误消息始终会被记录。
	Verbosity logsapi.VerbosityLevel `json:"verbosity" mapstructure:"verbosity"`
}

// NewLogOptions 创建一个使用默认参数的 Options 对象。
func NewLogOptions() *LogOptions {
	c := logsapi.LoggingConfiguration{}
	logsapi.SetRecommendedLoggingConfiguration(&c)

	return &LogOptions{
		Format:         c.Format,
		FlushFrequency: c.FlushFrequency.Duration.Duration,
		Verbosity:      c.Verbosity,
	}
}

// Validate 校验传递给 LogOptions 的命令行标志。
func (o *LogOptions) Validate() []error {
	errs := []error{}

	return errs
}

// AddFlags 为该配置添加命令行标志。
func (o *LogOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Format, fullPrefix+".format", o.Format, "设置日志格式。允许的格式：json、text。")
	fs.DurationVar(&o.FlushFrequency, fullPrefix+".flush-frequency", o.FlushFrequency, "日志刷新之间的最大秒数。")
	fs.VarP(logsapi.VerbosityLevelPflag(&o.Verbosity), fullPrefix+".verbosity", "", " 日志级别详细度的数值。")
}

func (o *LogOptions) Native() *logsapi.LoggingConfiguration {
	c := logsapi.LoggingConfiguration{}
	logsapi.SetRecommendedLoggingConfiguration(&c)
	c.Format = o.Format
	if o.FlushFrequency != 0 {
		c.FlushFrequency.Duration.Duration = o.FlushFrequency
		c.FlushFrequency.SerializeAsString = true
	}
	c.Verbosity = o.Verbosity
	return &c
}
