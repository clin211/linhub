package options

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

var _ IOptions = (*TLSOptions)(nil)

// TLSOptions 是用于提供安全（HTTPS）流量的 TLS 证书信息。
type TLSOptions struct {
	// UseTLS 指定是否应在可能的情况下使用 TLS 加密。
	UseTLS             bool   `json:"use-tls" mapstructure:"use-tls"`
	InsecureSkipVerify bool   `json:"insecure-skip-verify" mapstructure:"insecure-skip-verify"`
	CaCert             string `json:"ca-cert" mapstructure:"ca-cert"`
	Cert               string `json:"cert" mapstructure:"cert"`
	Key                string `json:"key" mapstructure:"key"`
}

// NewTLSOptions 创建一个 `zero` 值实例。
func NewTLSOptions() *TLSOptions {
	return &TLSOptions{}
}

// Validate 校验传递给 TLSOptions 的命令行标志。
func (o *TLSOptions) Validate() []error {
	errs := []error{}

	if !o.UseTLS {
		return errs
	}

	if (o.Cert != "" && o.Key == "") || (o.Cert == "" && o.Key != "") {
		errs = append(errs, fmt.Errorf("only one of cert and key configuration option is setted, you should set both to enable tls"))
	}

	return errs
}

// AddFlags 将与特定 APIServer 的 TLS 相关的命令行标志添加到指定的 FlagSet。
func (o *TLSOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.BoolVar(&o.UseTLS, fullPrefix+".use-tls", o.UseTLS, "使用 TLS 传输连接服务器。")
	fs.BoolVar(&o.InsecureSkipVerify, fullPrefix+".insecure-skip-verify", o.InsecureSkipVerify, ""+
		"控制客户端是否校验服务器的证书链和主机名。")
	fs.StringVar(&o.CaCert, fullPrefix+".ca-cert", o.CaCert, "用于连接服务器的 CA 证书路径。")
	fs.StringVar(&o.Cert, fullPrefix+".cert", o.Cert, "用于连接服务器的证书文件路径。")
	fs.StringVar(&o.Key, fullPrefix+".key", o.Key, "用于连接服务器的密钥文件路径。")
}

// MustTLSConfig 返回 TLS 配置，如果加载失败将 panic。
// 调用方必须保证证书路径合法；否则请使用 TLSConfig() 自行处理错误。
func (o *TLSOptions) MustTLSConfig() *tls.Config {
	tlsConf, err := o.TLSConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load TLS configuration: %w", err))
	}
	return tlsConf
}

func (o *TLSOptions) TLSConfig() (*tls.Config, error) {
	if !o.UseTLS {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: o.InsecureSkipVerify,
	}

	if o.Cert != "" && o.Key != "" {
		var cert tls.Certificate
		cert, err := tls.LoadX509KeyPair(o.Cert, o.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to loading tls certificates: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if o.CaCert != "" {
		data, err := os.ReadFile(o.CaCert)
		if err != nil {
			return nil, err
		}

		capool := x509.NewCertPool()
		for {
			var block *pem.Block
			block, _ = pem.Decode(data)
			if block == nil {
				break
			}
			cacert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			capool.AddCert(cacert)
		}

		tlsConfig.RootCAs = capool
	}

	return tlsConfig, nil
}

// Scheme 根据 TLS 配置返回相应的 URL scheme。
func (o *TLSOptions) Scheme() string {
	if o.UseTLS {
		return "https"
	}
	return "http"
}
