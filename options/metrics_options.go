package options

import (
	"github.com/jinzhu/copier"
	"github.com/spf13/pflag"
	"k8s.io/component-base/metrics"
)

var _ IOptions = (*MetricsOptions)(nil)

// MetricsOptions 包含从组件中暴露指标所需的所有参数。
type MetricsOptions struct {
	ShowHiddenMetricsForVersion string            `json:"show-hidden-metrics-for-version" mapstructure:"show-hidden-metrics-for-version"`
	DisabledMetrics             []string          `json:"disabled-metrics" mapstructure:"disabled-metrics"`
	AllowListMapping            map[string]string `json:"allow-metric-labels" mapstructure:"allow-metric-labels"`
}

// NewMetricsOptions 返回默认的指标选项。
func NewMetricsOptions() *MetricsOptions {
	opts := metrics.NewOptions()

	var o MetricsOptions
	_ = copier.Copy(&o, &opts)
	return &o
}

func (o *MetricsOptions) Native() *metrics.Options {
	var opts metrics.Options
	_ = copier.Copy(&opts, &o)
	return &opts
}

// Validate 校验指标命令行标志选项。
func (o *MetricsOptions) Validate() []error {
	return o.Native().Validate()
}

// AddFlags 添加用于暴露组件指标的命令行标志。
func (o *MetricsOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.ShowHiddenMetricsForVersion, fullPrefix+".show-hidden-metrics-for-version", o.ShowHiddenMetricsForVersion,
		"用于显示隐藏指标的旧版本。"+
			"只有上一个 minor 版本才有意义，其他值都不被允许。"+
			"格式为 <major>.<minor>，例如：'1.16'。"+
			"该格式的目的是确保你有机会注意到下一个版本是否隐藏了更多指标，"+
			"而不是在此后版本中它们被永久移除时感到惊讶。")
	fs.StringSliceVar(&o.DisabledMetrics,
		fullPrefix+".disabled-metrics",
		o.DisabledMetrics,
		"该标志为行为异常的指标提供了一种应急机制。"+
			"必须提供完整的指标名称才能禁用它。"+
			"免责声明：禁用指标的优先级高于显示隐藏指标。")
	fs.StringToStringVar(&o.AllowListMapping, fullPrefix+".allow-metric-labels", o.AllowListMapping,
		"从指标标签到该标签的允许值列表的映射。键的格式为 <MetricName>,<LabelName>。"+
			"值的格式为 <allowed_value>,<allowed_value>..."+
			"例如：metric1,label1='v1,v2,v3', metric1,label2='v1,v2,v3' metric2,label1='v1,v2,v3'。")
}
