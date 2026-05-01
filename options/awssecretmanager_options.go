package options

import (
	"fmt"
	"regexp"

	"github.com/spf13/pflag"
)

// 确保实现了 IOptions 接口
var _ IOptions = (*AWSSecretManagerOptions)(nil)

// AWSSecretManagerOptions 包含从 AWS Secrets Manager 读取 secret 所需的最小配置。
type AWSSecretManagerOptions struct {
	// Region 是 secret 所在的 AWS 区域，例如 "ap-southeast-1"。
	Region string `json:"region" mapstructure:"region"`

	// SecretName 是 AWS Secrets Manager 中 secret 的标识符，
	// 例如 "sre-redis-platform/-/ro"。
	SecretName string `json:"secret-name" mapstructure:"secret-name"`
}

// NewAWSSecretManagerOptions 创建一个使用合理默认值的选项实例。
func NewAWSSecretManagerOptions() *AWSSecretManagerOptions {
	return &AWSSecretManagerOptions{
		Region:     "ap-southeast-1",
		SecretName: "",
	}
}

// Validate 检查必填字段和基本约束。
func (o *AWSSecretManagerOptions) Validate() []error {
	if o == nil {
		return nil
	}
	var errs []error

	if o.Region == "" {
		errs = append(errs, fmt.Errorf("awssm.region is required"))
	} else {
		// 宽松的合理性检查，接受 us-east-1、ap-southeast-1、eu-west-3 等标准模式
		re := regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d$`)
		if !re.MatchString(o.Region) {
			// 不阻止非标准 partition，仅通过错误文本进行提醒
			errs = append(errs, fmt.Errorf("awssm.region %q looks unusual; expected like 'ap-southeast-1' or 'us-east-1'", o.Region))
		}
	}

	if o.SecretName == "" {
		errs = append(errs, fmt.Errorf("awssm.secret-name is required"))
	}

	return errs
}

// AddFlags 将 AWS Secrets Manager 相关的命令行标志注册到指定的 FlagSet，
// 使用 fullPrefix 作为标志名称的完整前缀。
func (o *AWSSecretManagerOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Region, fullPrefix+".region", o.Region,
		"secret 所在的 AWS 区域，例如：ap-southeast-1。")
	fs.StringVar(&o.SecretName, fullPrefix+".secret-name", o.SecretName,
		"AWS Secrets Manager 中 secret 的标识符，例如：sre-redis-platform/-/ro。")
}
