package token

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

// TestInit 测试 Init 函数（注意：Reset 后 key 为空，必须重新 Init）
func TestInit(t *testing.T) {
	Reset()

	// Reset 后默认 key 应为空（不再硬编码）
	assert.Equal(t, "", config.key)
	assert.Equal(t, "identityKey", config.identityKey)
	assert.Equal(t, 2*time.Hour, config.expiration)

	// 测试自定义配置
	Init("newKey", WithIdentityKey("newIdentityKey"), WithExpiration(3*time.Hour))
	assert.Equal(t, "newKey", config.key)
	assert.Equal(t, "newIdentityKey", config.identityKey)
	assert.Equal(t, 3*time.Hour, config.expiration)

	// 现在 Init 不再使用 sync.Once，二次调用会覆盖配置（行为变更）
	Init("anotherKey", WithIdentityKey("anotherIdentityKey"), WithExpiration(1*time.Hour))
	assert.Equal(t, "anotherKey", config.key)
	assert.Equal(t, "anotherIdentityKey", config.identityKey)
	assert.Equal(t, 1*time.Hour, config.expiration)

	// 为后续测试重置配置（必须显式 Init）
	Reset()
	Init("test-key-must-be-set-explicitly", WithIdentityKey("identityKey"))
}

// TestSign 测试 Sign 函数
func TestSign(t *testing.T) {
	Reset()
	Init("test-sign-key", WithIdentityKey("identityKey"))

	identityKey := "testUser"
	tokenString, _, err := Sign(identityKey)

	assert.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	parsedIdentityKey, err := ParseIdentity(tokenString, config.key)
	assert.NoError(t, err)
	assert.Equal(t, identityKey, parsedIdentityKey)
}

// TestParseInvalidToken 测试解析无效的 token
func TestParseInvalidToken(t *testing.T) {
	Reset()
	Init("test-parse-invalid-key", WithIdentityKey("identityKey"))

	invalidToken := "invalid.token.string"
	identityKey, err := ParseIdentity(invalidToken, config.key)

	assert.Error(t, err)
	assert.Empty(t, identityKey)
}

// TestParseRequestWithGin 测试从 Gin 上下文解析 token
func TestParseRequestWithGin(t *testing.T) {
	Reset()
	Init("test-parse-request-key", WithIdentityKey("identityKey"))

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer ")

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	identityKey, err := ParseRequest(c)
	assert.Error(t, err)
	assert.Empty(t, identityKey)

	validToken, _, _ := Sign("testUser")
	req.Header.Set("Authorization", "Bearer "+validToken)
	c.Request = req

	identityKey, err = ParseRequest(c)
	assert.NoError(t, err)
	assert.Equal(t, "testUser", identityKey)
}

// TestParseRequestWithGRPC 测试从 gRPC 上下文解析 token
func TestParseRequestWithGRPC(t *testing.T) {
	Reset()
	Init("test-grpc-key", WithIdentityKey("identityKey"))

	md := metadata.New(map[string]string{"Authorization": "Bearer "})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	identityKey, err := ParseRequest(ctx)
	assert.Error(t, err)
	assert.Empty(t, identityKey)

	validToken, _, _ := Sign("testUser")
	md = metadata.New(map[string]string{"Authorization": "Bearer " + validToken})
	ctx = metadata.NewIncomingContext(context.Background(), md)

	identityKey, err = ParseRequest(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "testUser", identityKey)
}
