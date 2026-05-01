package options

import (
	"github.com/spf13/pflag"
)

var _ IOptions = (*ClientCertAuthenticationOptions)(nil)

// ClientCertAuthenticationOptions 提供客户端证书认证的不同选项。
type ClientCertAuthenticationOptions struct {
	// ClientCA 是用于识别传入客户端证书的所有签发者的证书包
	ClientCA string `json:"client-ca-file" mapstructure:"client-ca-file"`
}

// NewClientCertAuthenticationOptions 创建一个使用默认参数的 ClientCertAuthenticationOptions 对象。
func NewClientCertAuthenticationOptions() *ClientCertAuthenticationOptions {
	return &ClientCertAuthenticationOptions{
		ClientCA: "",
	}
}

// Validate 用于解析和校验程序启动时用户在命令行输入的参数。
func (o *ClientCertAuthenticationOptions) Validate() []error {
	return []error{}
}

// AddFlags 将与特定服务器的 ClientCertAuthenticationOptions 相关的命令行标志添加到指定的 FlagSet。
func (o *ClientCertAuthenticationOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.ClientCA, fullPrefix+".ca-file", o.ClientCA, ""+
		"如果设置该项，任何请求只要提供由 client-ca-file 中的签发者之一所签发的客户端证书，"+
		"就会以对应于该客户端证书 CommonName 的身份完成认证。")
}
