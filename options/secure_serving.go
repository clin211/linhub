package options

import (
	"fmt"
	"path"

	"github.com/spf13/pflag"
)

var _ IOptions = (*SecureServingOptions)(nil)

// SecureServingOptions 包含与 HTTPS 服务器启动相关的配置项。
type SecureServingOptions struct {
	BindAddress string `json:"bind-address"`
	// 当设置了 Listener 时，BindPort 会被忽略；即使为 0 也会提供 HTTPS 服务。
	BindPort int `json:"bind-port"`
	// Required 设置为 true 表示 BindPort 不能为零。
	Required bool
	// ServerCert 是用于提供安全（HTTPS）流量的 TLS 证书信息
	ServerCert GeneratableKeyCert `json:"tls"`
	// AdvertiseAddress net.IP

	fullPrefix string
}

// CertKey 包含与证书相关的配置项。
type CertKey struct {
	// CertFile 是包含 PEM 编码证书的文件，可能还包含完整的证书链
	CertFile string `json:"cert-file"`
	// KeyFile 是包含 PEM 编码私钥的文件，对应于 CertFile 指定的证书
	KeyFile string `json:"private-key-file"`
}

// GeneratableKeyCert 包含与证书相关的配置项。
type GeneratableKeyCert struct {
	// CertKey 允许显式设置要使用的证书/密钥文件。
	CertKey CertKey `json:"cert-key"`

	// CertDirectory 指定在未显式设置 CertFile/KeyFile 时，写入生成证书的目录。
	// PairName 用于确定 CertDirectory 中的文件名。
	// 如果未设置 CertDirectory 和 PairName，则会在内存中生成一个证书。
	CertDirectory string `json:"cert-dir"`
	// PairName 是与 CertDirectory 一起用来构建证书和密钥文件名的名称。
	// 文件名将变为 CertDirectory/PairName.crt 和 CertDirectory/PairName.key
	PairName string `json:"pair-name"`
}

// NewSecureServingOptions 创建一个使用默认参数的 SecureServingOptions 对象。
func NewSecureServingOptions() *SecureServingOptions {
	return &SecureServingOptions{
		BindAddress: "0.0.0.0",
		BindPort:    8443,
		Required:    true,
		ServerCert: GeneratableKeyCert{
			PairName:      "onex",
			CertDirectory: "/var/run/onex",
		},
	}
}

// Validate 用于解析和校验程序启动时用户在命令行输入的参数。
func (s *SecureServingOptions) Validate() []error {
	if s == nil {
		return nil
	}

	errors := []error{}

	if s.Required && s.BindPort < 1 || s.BindPort > 65535 {
		errors = append(errors, fmt.Errorf("--"+s.fullPrefix+".bind-port %v must be between 1 and 65535, inclusive. It cannot be turned off with 0", s.BindPort))
	} else if s.BindPort < 0 || s.BindPort > 65535 {
		errors = append(errors, fmt.Errorf("--"+s.fullPrefix+".bind-port %v must be between 0 and 65535, inclusive. 0 for turning off secure port", s.BindPort))
	}

	return errors
}

// AddFlags 将与特定 APIServer 的 HTTPS 服务器相关的命令行标志添加到指定的 FlagSet。
func (s *SecureServingOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&s.BindAddress, fullPrefix+".bind-address", s.BindAddress, ""+
		"用于监听 --"+fullPrefix+".bind-port 端口的 IP 地址。"+
		"对应的网络接口必须可以被引擎其他部分以及 CLI/web 客户端访问。"+
		"如果为空，将使用所有网络接口（0.0.0.0 表示所有 IPv4 接口，:: 表示所有 IPv6 接口）。")
	desc := "提供带有认证和鉴权的 HTTPS 服务的端口。"
	if s.Required {
		desc += " 不能通过 0 关闭。"
	} else {
		desc += " 如果为 0，则完全不提供 HTTPS 服务。"
	}
	fs.IntVar(&s.BindPort, fullPrefix+".bind-port", s.BindPort, desc)

	fs.StringVar(&s.ServerCert.CertDirectory, fullPrefix+".tls.cert-dir", s.ServerCert.CertDirectory, ""+
		"TLS 证书所在的目录。"+
		"如果提供了 --"+fullPrefix+".tls.cert-key.cert-file 和 --"+fullPrefix+".tls.cert-key.private-key-file，"+
		"则此标志将被忽略。")

	fs.StringVar(&s.ServerCert.PairName, fullPrefix+".tls.pair-name", s.ServerCert.PairName, ""+
		"与 --"+fullPrefix+".tls.cert-dir 一起用来构建证书和密钥文件名的名称。"+
		"文件名将变为 <cert-dir>/<pair-name>.crt 和 <cert-dir>/<pair-name>.key")

	fs.StringVar(&s.ServerCert.CertKey.CertFile, fullPrefix+".tls.cert-key.cert-file", s.ServerCert.CertKey.CertFile, ""+
		"包含 HTTPS 默认 x509 证书的文件。（CA 证书（如有）将拼接在服务器证书之后）。")

	fs.StringVar(&s.ServerCert.CertKey.KeyFile, fullPrefix+".tls.cert-key.private-key-file",
		s.ServerCert.CertKey.KeyFile, ""+
			"包含与 --"+fullPrefix+".tls.cert-key.cert-file 匹配的默认 x509 私钥的文件。")
}

// Complete 补全那些未设置但需要有有效数据的字段。
func (s *SecureServingOptions) Complete() error {
	if s == nil || s.BindPort == 0 {
		return nil
	}

	keyCert := &s.ServerCert.CertKey
	if len(keyCert.CertFile) != 0 || len(keyCert.KeyFile) != 0 {
		return nil
	}

	if len(s.ServerCert.CertDirectory) > 0 {
		if len(s.ServerCert.PairName) == 0 {
			return fmt.Errorf("--%s.tls.pair-name is required if --%s.tls.cert-dir is set", s.fullPrefix, s.fullPrefix)
		}
		keyCert.CertFile = path.Join(s.ServerCert.CertDirectory, s.ServerCert.PairName+".crt")
		keyCert.KeyFile = path.Join(s.ServerCert.CertDirectory, s.ServerCert.PairName+".key")
	}

	return nil
}
