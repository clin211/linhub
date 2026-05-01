package options

import (
	"time"

	"github.com/spf13/pflag"
)

var _ IOptions = (*GRPCOptions)(nil)

// GRPCOptions 用于创建一个未认证、未授权、非安全的端口。
// 不应再有人使用这些选项。
type GRPCOptions struct {
	// Network 表示服务器网络类型。
	Network string `json:"network" mapstructure:"network"`

	// Addr 表示服务器地址。
	Addr string `json:"addr" mapstructure:"addr"`

	// Timeout 表示服务器超时时间。由 gRPC 客户端使用。
	Timeout time.Duration `json:"timeout" mapstructure:"timeout"`
}

// NewGRPCOptions 用于创建一个未认证、未授权、非安全的端口。
// 不应再有人使用这些选项。
func NewGRPCOptions() *GRPCOptions {
	return &GRPCOptions{
		Network: "tcp",
		Addr:    "0.0.0.0:39090",
		Timeout: 30 * time.Second,
	}
}

// Validate 用于解析和校验程序启动时用户在命令行输入的参数。
func (o *GRPCOptions) Validate() []error {
	var errors []error

	if err := ValidateAddress(o.Addr); err != nil {
		errors = append(errors, err)
	}

	return errors
}

// AddFlags 将与特定 API 服务器特性相关的命令行标志添加到指定的 FlagSet。
func (o *GRPCOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Network, fullPrefix+".network", o.Network, "指定 gRPC 服务器的网络类型。")
	fs.StringVar(&o.Addr, fullPrefix+".addr", o.Addr, "指定 gRPC 服务器的绑定地址和端口。")
	fs.DurationVar(&o.Timeout, fullPrefix+".timeout", o.Timeout, "服务器连接的超时时间。")
}
