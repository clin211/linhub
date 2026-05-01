package jwt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v4"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/clin211/linhub/authn"
	"github.com/clin211/linhub/i18n"
)

// hashToken 对 token 字符串做 SHA-256 哈希后转为十六进制，
// 用于在 store 中存储黑名单标识，避免明文 token 落库。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const (
	// reason 保存错误原因。
	reason string = "Unauthorized"
)

// 注意：先前版本的硬编码 defaultKey 已被移除，调用方必须通过 WithSigningKey 显式提供密钥。
// 若未显式设置而尝试 Sign，将返回 ErrSignTokenFailed。

// UnauthorizedError 表示一个未授权错误
type UnauthorizedError struct {
	Reason string
	Msg    string
}

func (e *UnauthorizedError) Error() string {
	return e.Msg
}

func unauthorized(reason, msg string) error {
	return &UnauthorizedError{Reason: reason, Msg: msg}
}

var (
	ErrTokenInvalid           = unauthorized(reason, "Token is invalid")
	ErrTokenExpired           = unauthorized(reason, "Token has expired")
	ErrTokenParseFail         = unauthorized(reason, "Fail to parse token")
	ErrUnSupportSigningMethod = unauthorized(reason, "Wrong signing method")
	ErrSignTokenFailed        = unauthorized(reason, "Failed to sign token")
)

// 定义 i18n 消息。
var (
	MessageTokenInvalid           = &goi18n.Message{ID: "jwt.token.invalid", Other: ErrTokenInvalid.Error()}
	MessageTokenExpired           = &goi18n.Message{ID: "jwt.token.expired", Other: ErrTokenExpired.Error()}
	MessageTokenParseFail         = &goi18n.Message{ID: "jwt.token.parse.failed", Other: ErrTokenParseFail.Error()}
	MessageUnSupportSigningMethod = &goi18n.Message{ID: "jwt.wrong.signing.method", Other: ErrUnSupportSigningMethod.Error()}
	MessageSignTokenFailed        = &goi18n.Message{ID: "jwt.token.sign.failed", Other: ErrSignTokenFailed.Error()}
)

// defaultOptions 提供 token 类型、有效期等默认值，但**不**提供默认 signingKey。
// 调用方必须通过 WithSigningKey 显式设置密钥，否则 Sign 会失败。
var defaultOptions = options{
	tokenType:     "Bearer",
	expired:       2 * time.Hour,
	signingMethod: jwt.SigningMethodHS256,
	signingKey:    nil,
	keyfunc:       nil,
}

type options struct {
	signingMethod jwt.SigningMethod
	signingKey    any
	keyfunc       jwt.Keyfunc
	issuer        string
	expired       time.Duration
	tokenType     string
	tokenHeader   map[string]any
}

// Option 是 jwt 的选项。
type Option func(*options)

// WithSigningMethod 设置签名方法。
func WithSigningMethod(method jwt.SigningMethod) Option {
	return func(o *options) {
		o.signingMethod = method
	}
}

// WithIssuer 设置 token 的颁发者，用于标识签发该 JWT 的主体。
func WithIssuer(issuer string) Option {
	return func(o *options) {
		o.issuer = issuer
	}
}

// WithSigningKey 设置签名密钥。
func WithSigningKey(key any) Option {
	return func(o *options) {
		o.signingKey = key
	}
}

// WithKeyfunc 设置用于验证密钥的回调函数。
func WithKeyfunc(keyFunc jwt.Keyfunc) Option {
	return func(o *options) {
		o.keyfunc = keyFunc
	}
}

// WithExpired 设置 token 的过期时间（单位为秒，默认 2 小时）。
func WithExpired(expired time.Duration) Option {
	return func(o *options) {
		o.expired = expired
	}
}

// WithTokenHeader 为客户端设置自定义的 tokenHeader。
func WithTokenHeader(header map[string]any) Option {
	return func(o *options) {
		o.tokenHeader = header
	}
}

// New 创建一个认证器实例。
// 必须通过 WithSigningKey 提供签名密钥，未提供时 keyfunc 会基于 signingKey 自动构造。
func New(store Storer, opts ...Option) *JWTAuth {
	o := defaultOptions
	for _, opt := range opts {
		opt(&o)
	}

	if o.keyfunc == nil && o.signingKey != nil {
		signingKey := o.signingKey
		o.keyfunc = func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrTokenInvalid
			}
			return signingKey, nil
		}
	}

	return &JWTAuth{opts: &o, store: store}
}

// JWTAuth 实现了 authn.Authenticator 接口。
type JWTAuth struct {
	opts  *options
	store Storer
}

// Sign 用于生成一个 token。
func (a *JWTAuth) Sign(ctx context.Context, userID string) (authn.IToken, error) {
	now := time.Now()
	expiresAt := now.Add(a.opts.expired)

	token := jwt.NewWithClaims(a.opts.signingMethod, &jwt.RegisteredClaims{
		// Issuer = iss,令牌颁发者。它表示该令牌是由谁创建的
		Issuer: a.opts.issuer,
		// IssuedAt = iat,令牌颁发时的时间戳。它表示令牌是何时被创建的
		IssuedAt: jwt.NewNumericDate(now),
		// ExpiresAt = exp,令牌的过期时间戳。它表示令牌将在何时过期
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		// NotBefore = nbf,令牌的生效时的时间戳。它表示令牌从什么时候开始生效
		NotBefore: jwt.NewNumericDate(now),
		// Subject = sub,令牌的主体。它表示该令牌是关于谁的
		Subject: userID,
	})
	if a.opts.tokenHeader != nil {
		for k, v := range a.opts.tokenHeader {
			token.Header[k] = v
		}
	}

	refreshToken, err := token.SignedString(a.opts.signingKey)
	if err != nil {
		return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageSignTokenFailed))
	}

	tokenInfo := &tokenInfo{
		ExpiresAt: expiresAt.Unix(),
		Type:      a.opts.tokenType,
		Token:     refreshToken,
	}

	return tokenInfo, nil
}

// parseToken 用于解析传入的 refreshToken。
func (a *JWTAuth) parseToken(ctx context.Context, refreshToken string) (*jwt.RegisteredClaims, error) {
	token, err := jwt.ParseWithClaims(refreshToken, &jwt.RegisteredClaims{}, a.opts.keyfunc)
	if err != nil {
		ve, ok := err.(*jwt.ValidationError)
		if !ok {
			return nil, unauthorized(reason, err.Error())
		}
		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageTokenInvalid))
		}
		if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
			return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageTokenExpired))
		}
		return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageTokenParseFail))
	}

	if !token.Valid {
		return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageTokenInvalid))
	}

	if token.Method != a.opts.signingMethod {
		return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageUnSupportSigningMethod))
	}

	return token.Claims.(*jwt.RegisteredClaims), nil
}

func (a *JWTAuth) callStore(fn func(Storer) error) error {
	if store := a.store; store != nil {
		return fn(store)
	}
	return nil
}

// Destroy 用于销毁一个 token，将其哈希后写入黑名单 store。
func (a *JWTAuth) Destroy(ctx context.Context, refreshToken string) error {
	claims, err := a.parseToken(ctx, refreshToken)
	if err != nil {
		return err
	}

	tokenHash := hashToken(refreshToken)

	store := func(store Storer) error {
		expired := time.Until(claims.ExpiresAt.Time)
		if expired <= 0 {
			return nil
		}
		return store.Set(ctx, tokenHash, expired)
	}
	return a.callStore(store)
}

// ParseClaims 解析 token 并返回 claims。
// 若 token 在 store 黑名单中（按哈希查找），返回 ErrTokenInvalid。
func (a *JWTAuth) ParseClaims(ctx context.Context, refreshToken string) (*jwt.RegisteredClaims, error) {
	if refreshToken == "" {
		return nil, unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageTokenInvalid))
	}

	claims, err := a.parseToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	tokenHash := hashToken(refreshToken)

	store := func(store Storer) error {
		exists, err := store.Check(ctx, tokenHash)
		if err != nil {
			return err
		}

		if exists {
			return unauthorized(reason, i18n.FromContext(ctx).LocalizeT(MessageTokenInvalid))
		}

		return nil
	}

	if err := a.callStore(store); err != nil {
		return nil, err
	}

	return claims, nil
}

// Release 用于释放申请的资源。
func (a *JWTAuth) Release() error {
	return a.callStore(func(store Storer) error {
		return store.Close()
	})
}
