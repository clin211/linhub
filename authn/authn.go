package authn

import (
	"context"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// IToken 定义了实现通用 token 所需的方法。
type IToken interface {
	// 获取 token 字符串。
	GetToken() string
	// 获取 token 类型。
	GetTokenType() string
	// 获取 token 过期时间戳。
	GetExpiresAt() int64
	// JSON 编码
	EncodeToJSON() ([]byte, error)
}

// Authenticator 定义了用于 token 处理的方法。
type Authenticator interface {
	// Sign 用于生成一个 token。
	Sign(ctx context.Context, userID string) (IToken, error)

	// Destroy 用于销毁一个 token。
	Destroy(ctx context.Context, accessToken string) error

	// ParseClaims 解析 token 并返回其 claims。
	ParseClaims(ctx context.Context, accessToken string) (*jwt.RegisteredClaims, error)

	// Release 用于释放申请的资源。
	Release() error
}

// 默认 bcrypt cost。bcrypt.DefaultCost=10 在 2024+ 已偏低，
// 我们提升为 12 以提高对暴力破解的成本（每次 +1 计算量翻倍）。
const DefaultBcryptCost = 12

// Encrypt 使用 bcrypt 对明文进行加密（使用 DefaultBcryptCost）。
func Encrypt(source string) (string, error) {
	return EncryptWithCost(source, DefaultBcryptCost)
}

// EncryptWithCost 使用指定 cost 对明文进行 bcrypt 加密。
// cost 范围：bcrypt.MinCost..bcrypt.MaxCost，超出范围时回退到 DefaultBcryptCost。
func EncryptWithCost(source string, cost int) (string, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = DefaultBcryptCost
	}
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(source), cost)
	return string(hashedBytes), err
}

// Compare 比较加密后的文本与明文是否相同。
func Compare(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
