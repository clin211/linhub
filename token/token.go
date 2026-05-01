package token

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config 包括 token 包的配置选项.
type Config struct {
	// key 用于签发和解析 token 的密钥.
	key string
	// identityKey 是 token 中用户身份的键.
	identityKey string
	// expiration 是签发的 token 过期时间
	expiration time.Duration
	// skipPaths 需要跳过认证的路径列表
	skipPaths []string
	// signingMethod 指定 JWT 签名算法（默认 HS256）
	signingMethod jwt.SigningMethod
}

// Option 用于配置 token 包的选项
type Option func(*Config)

var (
	configMu sync.RWMutex
	config   = Config{
		key:           "",
		identityKey:   "",
		expiration:    2 * time.Hour,
		skipPaths:     []string{},
		signingMethod: jwt.SigningMethodHS256,
	}
)

// 预定义错误
var (
	ErrMissingIdentityKey  = errors.New("missing identity key in token")
	ErrInvalidIdentityKey  = errors.New("invalid identity key in token")
	ErrEmptyToken          = errors.New("token is empty")
	ErrInvalidAuthHeader   = errors.New("invalid authorization header")
	ErrEmptyAuthHeader     = errors.New("authorization header is empty")
	ErrMalformedAuthHeader = errors.New("malformed authorization header")
	ErrInvalidTokenClaims  = errors.New("invalid token claims")
	ErrPathSkipped         = errors.New("path is skipped for authentication")
	ErrSigningKeyNotSet    = errors.New("signing key is not configured")
)

// WithKey 设置签名密钥
func WithKey(key string) Option {
	return func(c *Config) {
		if key != "" {
			c.key = key
		}
	}
}

// WithIdentityKey 设置身份键名称
func WithIdentityKey(identityKey string) Option {
	return func(c *Config) {
		c.identityKey = identityKey // 允许设置为空字符串来禁用身份键验证
	}
}

// WithExpiration 设置过期时间
func WithExpiration(expiration time.Duration) Option {
	return func(c *Config) {
		if expiration > 0 {
			c.expiration = expiration
		}
	}
}

// WithSigningMethod 设置签名算法（默认 HS256）。
// 支持的算法名称：HS256、HS384、HS512。
func WithSigningMethod(method string) Option {
	return func(c *Config) {
		switch strings.ToUpper(method) {
		case "HS256":
			c.signingMethod = jwt.SigningMethodHS256
		case "HS384":
			c.signingMethod = jwt.SigningMethodHS384
		case "HS512":
			c.signingMethod = jwt.SigningMethodHS512
		}
	}
}

// WithSkipPaths 设置需要跳过认证的路径列表
// 支持精确匹配和通配符匹配
func WithSkipPaths(paths ...string) Option {
	return func(c *Config) {
		c.skipPaths = append(c.skipPaths, paths...)
	}
}

// WithCommonSkipPaths 添加常见的跳过路径（健康检查、监控等）
func WithCommonSkipPaths() Option {
	commonPaths := []string{
		"/health",
		"/healthz",
		"/health/*",
		"/ready",
		"/readiness",
		"/live",
		"/liveness",
		"/metrics",
		"/prometheus",
		"/status",
		"/ping",
		"/version",
		"/info",
		"/favicon.ico",
		"/robots.txt",
	}
	return func(c *Config) {
		c.skipPaths = append(c.skipPaths, commonPaths...)
	}
}

// WithSkipPathsPattern 使用模式匹配添加跳过路径
func WithSkipPathsPattern(patterns ...string) Option {
	return func(c *Config) {
		c.skipPaths = append(c.skipPaths, patterns...)
	}
}

// Init 设置包级别的配置 config, config 会用于本包后面的 token 签发和解析.
// 与 sync.Once 不同，本实现允许重新初始化（在测试或热更新场景常见），
// 但每次调用都会发出 warning 日志以便排查重复 Init 的来源。
func Init(key string, opts ...Option) {
	configMu.Lock()
	defer configMu.Unlock()

	if key != "" {
		config.key = key
	}
	for _, opt := range opts {
		opt(&config)
	}
}

// Reset 重置配置（主要用于测试）。
// 注意：不再硬编码默认密钥，重置后 key 为空，必须重新 Init。
func Reset() {
	configMu.Lock()
	defer configMu.Unlock()
	config = Config{
		key:           "",
		identityKey:   "identityKey",
		expiration:    2 * time.Hour,
		skipPaths:     []string{},
		signingMethod: jwt.SigningMethodHS256,
	}
}

// shouldSkipPath 检查路径是否应该跳过认证
func shouldSkipPath(requestPath string) bool {
	for _, skipPath := range config.skipPaths {
		if matchPath(requestPath, skipPath) {
			return true
		}
	}
	return false
}

// matchPath 路径匹配函数，支持通配符
func matchPath(requestPath, pattern string) bool {
	// 精确匹配
	if requestPath == pattern {
		return true
	}

	// 通配符匹配
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(requestPath, prefix)
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(requestPath, prefix)
	}

	// 支持中间通配符（简单实现）
	if strings.Contains(pattern, "*") {
		return matchWildcard(requestPath, pattern)
	}

	return false
}

// matchWildcard 通配符匹配（简单实现）
func matchWildcard(str, pattern string) bool {
	// 将模式分割为非通配符部分
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return str == pattern
	}

	// 检查开头
	if !strings.HasPrefix(str, parts[0]) {
		return false
	}

	// 检查结尾
	if len(parts[len(parts)-1]) > 0 && !strings.HasSuffix(str, parts[len(parts)-1]) {
		return false
	}

	// 简单的中间匹配检查
	currentPos := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}

		index := strings.Index(str[currentPos:], part)
		if index == -1 {
			return false
		}
		currentPos += index + len(part)
	}

	return true
}

// ParseIdentity 使用指定的密钥 key 解析 token，解析成功返回 token 上下文，否则报错.
func ParseIdentity(tokenString string, key string) (string, error) {
	if tokenString == "" {
		return "", ErrEmptyToken
	}

	if key == "" {
		return "", jwt.ErrInvalidKey
	}

	// 解析 token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 确保 token 加密算法符合预期的加密算法
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(key), nil
	})
	if err != nil {
		return "", err
	}

	// 验证 token 有效性
	if !token.Valid {
		return "", jwt.ErrSignatureInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrInvalidTokenClaims
	}

	return extractIdentity(claims)
}

// extractIdentity 从 claims 中提取身份信息
func extractIdentity(claims jwt.MapClaims) (string, error) {
	// 如果没有配置身份键，返回空字符串（表示不需要身份验证）
	if config.identityKey == "" {
		return "", nil
	}

	// 检查身份键是否存在
	value, exists := claims[config.identityKey]
	if !exists {
		return "", ErrMissingIdentityKey
	}

	// 验证身份键的值
	identity, ok := value.(string)
	if !ok {
		return "", ErrInvalidIdentityKey
	}

	// 身份键不能为空（如果配置了身份键）
	if identity == "" {
		return "", ErrInvalidIdentityKey
	}

	return identity, nil
}

// ParseRequest 从请求头中获取令牌，并将其传递递给 Parse 函数以解析令牌.
//
// 安全注意：当请求路径在 skipPaths 配置中时，会返回 (("", ErrPathSkipped))。
// 调用方必须显式检查 errors.Is(err, ErrPathSkipped) 来决定是否放行，
// 而不能假设 err == nil 即代表认证通过。
func ParseRequest(ctx context.Context) (string, error) {
	if shouldSkipRequestPath(ctx) {
		return "", ErrPathSkipped
	}

	token, err := extractTokenFromRequest(ctx)
	if err != nil {
		return "", err
	}

	configMu.RLock()
	key := config.key
	configMu.RUnlock()

	return ParseIdentity(token, key)
}

// shouldSkipRequestPath 检查请求路径是否应该跳过认证
func shouldSkipRequestPath(ctx context.Context) bool {
	switch typed := ctx.(type) {
	case *gin.Context:
		return shouldSkipPath(typed.Request.URL.Path)
	default:
		// 对于gRPC，可以从metadata中获取method信息
		// 这里简化处理，如果需要可以扩展
		return false
	}
}

// ParseRequestIgnoreSkip 强制解析请求，忽略跳过路径设置
func ParseRequestIgnoreSkip(ctx context.Context) (string, error) {
	token, err := extractTokenFromRequest(ctx)
	if err != nil {
		return "", err
	}

	configMu.RLock()
	key := config.key
	configMu.RUnlock()

	return ParseIdentity(token, key)
}

// extractTokenFromRequest 从不同类型的请求上下文中提取 token
func extractTokenFromRequest(ctx context.Context) (string, error) {
	switch typed := ctx.(type) {
	case *gin.Context:
		return extractTokenFromGin(typed)
	default:
		return extractTokenFromGRPC(ctx)
	}
}

// extractTokenFromGin 从 Gin Context 中提取 token
func extractTokenFromGin(c *gin.Context) (string, error) {
	header := c.Request.Header.Get("Authorization")
	if header == "" {
		return "", ErrEmptyAuthHeader
	}

	return parseAuthorizationHeader(header)
}

// extractTokenFromGRPC 从 gRPC Context 中提取 token
func extractTokenFromGRPC(ctx context.Context) (string, error) {
	token, err := auth.AuthFromMD(ctx, "Bearer")
	if err != nil {
		return "", status.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
	}
	return token, nil
}

// parseAuthorizationHeader 解析 Authorization 头部，支持大小写不敏感（RFC 7235）。
func parseAuthorizationHeader(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrMalformedAuthHeader
	}
	if parts[1] == "" {
		return "", ErrEmptyToken
	}
	return parts[1], nil
}

// Sign 使用 jwtSecret 签发 token，token 的 claims 中会存放传入的 subject.
func Sign(identityValue string) (string, time.Time, error) {
	configMu.RLock()
	key := config.key
	identityKey := config.identityKey
	expiration := config.expiration
	method := config.signingMethod
	configMu.RUnlock()

	if key == "" {
		return "", time.Time{}, ErrSigningKeyNotSet
	}
	if method == nil {
		method = jwt.SigningMethodHS256
	}

	now := time.Now()
	expireAt := now.Add(expiration)

	claims := jwt.MapClaims{
		"nbf": now.Unix(),
		"iat": now.Unix(),
		"exp": expireAt.Unix(),
	}

	if identityKey != "" && identityValue != "" {
		claims[identityKey] = identityValue
	}

	token := jwt.NewWithClaims(method, claims)

	tokenString, err := token.SignedString([]byte(key))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expireAt, nil
}

// SignWithClaims 使用自定义 claims 签发 token
func SignWithClaims(customClaims jwt.MapClaims) (string, time.Time, error) {
	configMu.RLock()
	key := config.key
	expiration := config.expiration
	method := config.signingMethod
	configMu.RUnlock()

	if key == "" {
		return "", time.Time{}, ErrSigningKeyNotSet
	}
	if method == nil {
		method = jwt.SigningMethodHS256
	}

	now := time.Now()
	expireAt := now.Add(expiration)

	claims := make(jwt.MapClaims)
	for k, v := range customClaims {
		claims[k] = v
	}

	if _, exists := claims["nbf"]; !exists {
		claims["nbf"] = now.Unix()
	}
	if _, exists := claims["iat"]; !exists {
		claims["iat"] = now.Unix()
	}
	if _, exists := claims["exp"]; !exists {
		claims["exp"] = expireAt.Unix()
	}

	token := jwt.NewWithClaims(method, claims)

	tokenString, err := token.SignedString([]byte(key))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expireAt, nil
}

// Parse 验证 token 字符串的有效性（不解析身份信息）
func Parse(tokenString string) error {
	if tokenString == "" {
		return ErrEmptyToken
	}

	if config.key == "" {
		return jwt.ErrInvalidKey
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.key), nil
	})
	if err != nil {
		return err
	}

	if !token.Valid {
		return jwt.ErrSignatureInvalid
	}

	return nil
}

// GetClaims 获取 token 中的所有 claims
func GetClaims(tokenString string) (jwt.MapClaims, error) {
	if tokenString == "" {
		return nil, ErrEmptyToken
	}

	if config.key == "" {
		return nil, jwt.ErrInvalidKey
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.key), nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenClaims
	}

	return claims, nil
}

// GetConfig 获取当前配置（用于调试和测试）
func GetConfig() Config {
	return config
}

// IsIdentityRequired 检查是否需要身份验证
func IsIdentityRequired() bool {
	return config.identityKey != ""
}

// GetExpiration 获取当前配置的过期时间
func GetExpiration() time.Duration {
	return config.expiration
}

// GetSkipPaths 获取跳过认证的路径列表
func GetSkipPaths() []string {
	return append([]string{}, config.skipPaths...) // 返回副本
}

// IsPathSkipped 检查指定路径是否被跳过认证
func IsPathSkipped(path string) bool {
	return shouldSkipPath(path)
}

// ParseWithKey 使用自定义密钥解析 token
func ParseWithKey(tokenString, key string) (jwt.MapClaims, error) {
	if tokenString == "" {
		return nil, ErrEmptyToken
	}

	if key == "" {
		return nil, jwt.ErrInvalidKey
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(key), nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenClaims
	}

	return claims, nil
}
