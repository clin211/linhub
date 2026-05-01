package options

import (
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/spf13/pflag"
)

var _ IOptions = (*JWTOptions)(nil)

// JWTOptions 包含与 API 服务器特性相关的配置项。
type JWTOptions struct {
	Key           string        `json:"key" mapstructure:"key"`
	Expired       time.Duration `json:"expired" mapstructure:"expired"`
	MaxRefresh    time.Duration `json:"max-refresh" mapstructure:"max-refresh"`
	SigningMethod string        `json:"signing-method" mapstructure:"signing-method"`

	fullPrefix string
}

// NewJWTOptions 创建一个使用默认参数的 JWTOptions 对象。
// 注意：Key 默认为空，必须在配置文件或命令行中显式提供，
// 否则 Validate() 会失败，避免使用不安全的默认密钥。
func NewJWTOptions() *JWTOptions {
	return &JWTOptions{
		Key:           "",
		Expired:       2 * time.Hour,
		MaxRefresh:    2 * time.Hour,
		SigningMethod: "HS256",
	}
}

// Validate 用于解析和校验程序启动时用户在命令行输入的参数。
func (s *JWTOptions) Validate() []error {
	var errs []error

	if s.Key == "" {
		errs = append(errs, fmt.Errorf("--%s.key is required (must not be empty for security)", s.fullPrefix))
	} else if !govalidator.StringLength(s.Key, "32", "256") {
		errs = append(errs, fmt.Errorf("--%s.key must be at least 32 characters and at most 256 characters", s.fullPrefix))
	}

	switch s.SigningMethod {
	case "HS256", "HS384", "HS512":
	default:
		errs = append(errs, fmt.Errorf("--%s.signing-method %q is not supported (allowed: HS256/HS384/HS512)", s.fullPrefix, s.SigningMethod))
	}

	return errs
}

// AddFlags 将与特定 API 服务器特性相关的命令行标志添加到指定的 FlagSet。
func (s *JWTOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	if fs == nil {
		return
	}

	// fs.StringVar(&s.Realm, fullPrefix+".realm", s.Realm, "向用户展示的 Realm 名称。")
	fs.StringVar(&s.Key, fullPrefix+".key", s.Key, "用于签名 JWT token 的私钥。")
	fs.DurationVar(&s.Expired, fullPrefix+".expired", s.Expired, "JWT token 过期时间。")
	fs.DurationVar(&s.MaxRefresh, fullPrefix+".max-refresh", s.MaxRefresh, ""+
		"该字段允许客户端在 MaxRefresh 过去之前刷新其 token。")
	fs.StringVar(&s.SigningMethod, fullPrefix+".signing-method", s.SigningMethod, "JWT token 签名方法。")
}
