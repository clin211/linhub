package errx

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBizErrorBasic(t *testing.T) {
	err := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")

	assert.Equal(t, CodeUserNotFound, err.Code)
	assert.Equal(t, LevelUser, err.Level)
	assert.Equal(t, "User.NotFound", err.Reason)
	assert.Equal(t, "用户不存在", err.Message)
	assert.Equal(t, "biz error: code=20101 reason=User.NotFound message=用户不存在", err.Error())
}

func TestBizErrorErrorIncludesDetailsAndMetadata(t *testing.T) {
	err := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")

	withDetails := err.WithDetails("用户ID 12345 不存在")
	assert.Contains(t, withDetails.Error(), "details=用户ID 12345 不存在")

	withMeta := err.WithMetadata(map[string]any{"user_id": 12345})
	assert.Contains(t, withMeta.Error(), "metadata=")
}

func TestBizErrorWithMessage(t *testing.T) {
	originalErr := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")
	newErr := originalErr.WithMessage("用户账户已被删除")

	assert.Equal(t, "用户账户已被删除", newErr.Message)
	assert.Equal(t, originalErr.Code, newErr.Code)
	assert.Equal(t, originalErr.Reason, newErr.Reason)
	assert.Equal(t, originalErr.Level, newErr.Level)

	// 原错误 Message 不受影响
	assert.Equal(t, "用户不存在", originalErr.Message)
	assert.NotSame(t, originalErr, newErr)
}

func TestBizErrorWithMessagePreservesMetadata(t *testing.T) {
	originalErr := NewBizError(CodeUserInsufficientBalance, "User.InsufficientBalance", "余额不足").
		WithMetadata(map[string]any{
			"current_balance": 100.00,
			"required_amount": 200.00,
		})

	newErr := originalErr.WithMessage("余额不足，请充值").
		WithMetadata(map[string]any{"current_balance": 50.00})

	assert.Equal(t, 50.00, newErr.Metadata["current_balance"])
	assert.Nil(t, newErr.Metadata["required_amount"])

	// 原错误 metadata 不受影响
	assert.Equal(t, 100.00, originalErr.Metadata["current_balance"])
	assert.Equal(t, 200.00, originalErr.Metadata["required_amount"])
}

func TestGetErrorLevel(t *testing.T) {
	tests := []struct {
		code  BizCode
		level int
	}{
		{CodeOK, 0},
		{CodeInternalServer, LevelSystem},
		{CodeUserNotFound, LevelUser},
		{CodePostNotFound, LevelBusiness},
		{CodeAuthUnauthenticated, LevelUser},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.level, GetErrorLevel(tt.code), "code=%d", tt.code)
	}
}

func TestGetModule(t *testing.T) {
	tests := []struct {
		code   BizCode
		module int
	}{
		{CodeUserNotFound, ModuleUser},
		{CodePostNotFound, ModulePost},
		{CodeCommentNotFound, ModuleComment},
		{CodeAuthUnauthenticated, ModuleAuth},
		{CodeDatabaseReadFailed, ModuleDatabase},
		{CodeCacheReadFailed, ModuleCache},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.module, GetModule(tt.code), "code=%d", tt.code)
	}
}

// TestGetHTTPCode 验证关键的"业务错误返回 200"设计原则。
func TestGetHTTPCode(t *testing.T) {
	tests := []struct {
		name     string
		code     BizCode
		httpCode int
	}{
		// 成功
		{"success", CodeOK, http.StatusOK},

		// 业务错误（用户级 + 业务级）→ 一律 HTTP 200
		{"user not found is 200", CodeUserNotFound, http.StatusOK},
		{"unauthenticated is 200", CodeAuthUnauthenticated, http.StatusOK},
		{"token expired is 200", CodeAuthTokenExpired, http.StatusOK},
		{"permission denied is 200", CodeUserPermissionDenied, http.StatusOK},
		{"post not found is 200", CodePostNotFound, http.StatusOK},
		{"post forbidden is 200", CodePostPermissionDenied, http.StatusOK},
		{"too many requests is 200", CodeTooManyRequests, http.StatusOK},
		{"invalid parameter is 200", CodeInvalidParameter, http.StatusOK},

		// 系统级错误 → 5xx
		{"internal server is 500", CodeInternalServer, http.StatusInternalServerError},
		{"db connect failed is 500", CodeDatabaseConnectFailed, http.StatusInternalServerError},
		{"db read failed is 500", CodeDatabaseReadFailed, http.StatusInternalServerError},
		{"cache read failed is 500", CodeCacheReadFailed, http.StatusInternalServerError},
		{"service unavailable is 503", CodeServiceUnavailable, http.StatusServiceUnavailable},
		{"request timeout is 504", CodeRequestTimeout, http.StatusGatewayTimeout},
		{"sign token is 500", CodeAuthSignToken, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.httpCode, GetHTTPCode(tt.code), "code=%d", tt.code)
		})
	}
}

func TestAPIResponseSuccess(t *testing.T) {
	resp := Success(map[string]string{"name": "alice"}, "操作成功")
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "操作成功", resp.Message)
	assert.Equal(t, "alice", resp.Data.(map[string]string)["name"])

	// 默认 message
	resp = Success(nil)
	assert.Equal(t, "success", resp.Message)
}

func TestAPIResponseFailure(t *testing.T) {
	resp := Failure(20101, "用户不存在")
	assert.Equal(t, 20101, resp.Code)
	assert.Equal(t, "用户不存在", resp.Message)
	assert.Nil(t, resp.Data)

	resp.WithReason("User.NotFound").WithDetails("用户ID 12345 不存在")
	assert.Equal(t, "User.NotFound", resp.Reason)
	assert.Equal(t, "用户ID 12345 不存在", resp.Details)
}

func TestFromBizErrorPropagatesFields(t *testing.T) {
	bizErr := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在").
		WithDetails("用户ID 12345 不存在").
		WithMetadata(map[string]any{"user_id": 12345})

	resp := FromBizError(bizErr)

	assert.Equal(t, int(CodeUserNotFound), resp.Code)
	assert.Equal(t, "用户不存在", resp.Message)
	assert.Nil(t, resp.Data)
	assert.Equal(t, "User.NotFound", resp.Reason)
	assert.Equal(t, "用户ID 12345 不存在", resp.Details)
}

func TestFromError(t *testing.T) {
	bizErr := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")
	converted := FromError(bizErr)
	assert.Equal(t, bizErr, converted)

	normalErr := errors.New("normal error")
	converted = FromError(normalErr)
	assert.Equal(t, CodeInternalServer, converted.Code)
	assert.Equal(t, "Unknown", converted.Reason)
	assert.Equal(t, "normal error", converted.Message)

	assert.Nil(t, FromError(nil))
}

func TestCodeAndReason(t *testing.T) {
	assert.Equal(t, int(CodeOK), Code(nil))
	assert.Equal(t, "", Reason(nil))

	bizErr := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")
	assert.Equal(t, int(CodeUserNotFound), Code(bizErr))
	assert.Equal(t, "User.NotFound", Reason(bizErr))

	normalErr := errors.New("test error")
	assert.Equal(t, int(CodeInternalServer), Code(normalErr))
	assert.Equal(t, "InternalError", Reason(normalErr))
}

func TestPredefinedErrors(t *testing.T) {
	// 验证预定义错误的 HTTP 状态码符合"业务错误 200"设计
	tests := []struct {
		name     string
		err      *BizError
		httpCode int
	}{
		{"ErrUnauthenticated", ErrUnauthenticated, http.StatusOK},
		{"ErrTokenInvalid", ErrTokenInvalid, http.StatusOK},
		{"ErrTokenExpired", ErrTokenExpired, http.StatusOK},
		{"ErrPermissionDenied", ErrPermissionDenied, http.StatusOK},
		{"ErrNotFound", ErrNotFound, http.StatusOK},
		{"ErrBind", ErrBind, http.StatusOK},
		{"ErrInvalidArgument", ErrInvalidArgument, http.StatusOK},
		{"ErrTooManyRequests", ErrTooManyRequests, http.StatusOK},

		{"ErrInternal", ErrInternal, http.StatusInternalServerError},
		{"ErrServiceUnavailable", ErrServiceUnavailable, http.StatusServiceUnavailable},
		{"ErrRequestTimeout", ErrRequestTimeout, http.StatusGatewayTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.httpCode, GetHTTPCode(tt.err.Code))
		})
	}
}

func BenchmarkNewBizError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")
	}
}

func BenchmarkFromError(b *testing.B) {
	err := NewBizError(CodeUserNotFound, "User.NotFound", "用户不存在")
	for i := 0; i < b.N; i++ {
		_ = FromError(err)
	}
}
