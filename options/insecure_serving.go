package options

import (
	"github.com/spf13/pflag"
)

var _ IOptions = (*InsecureServingOptions)(nil)

// InsecureServingOptions 用于创建一个未认证、未授权、非安全的端口。
// 不应再有人使用这些选项。
type InsecureServingOptions struct {
	Addr string `json:"addr" mapstructure:"addr"`
}

// NewInsecureServingOptions 用于创建一个未认证、未授权、非安全的端口。
// 不应再有人使用这些选项。
func NewInsecureServingOptions() *InsecureServingOptions {
	return &InsecureServingOptions{
		Addr: "127.0.0.1:8080",
	}
}

// Validate 用于解析和校验程序启动时用户在命令行输入的参数。
func (s *InsecureServingOptions) Validate() []error {
	var errors []error

	return errors
}

// AddFlags 将与特定 API 服务器特性相关的命令行标志添加到指定的 FlagSet。
func (s *InsecureServingOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&s.Addr, fullPrefix+".addr", s.Addr, "指定 HTTP 服务器的绑定地址和端口。")
}
