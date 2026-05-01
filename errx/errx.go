// Package errx 提供基于业务错误码的统一错误处理。
//
// 设计原则（前后端分离最佳实践）：
//
//  1. 任何能成功到达业务层的请求都返回 HTTP 200 状态码，
//     业务结果通过响应 body 中的 `code` 字段进行区分。
//     这样前端只需识别 body 中的 code 即可统一处理：
//     - code == 0 → 业务成功
//     - code != 0 → 业务失败，详细信息见 message/details/metadata
//
//  2. 仅当请求无法被正常处理（系统级故障）时才返回非 200 的状态码：
//     - 5xx：服务端内部错误、上游不可达、超时等
//     - 502/503/504：网关、服务不可达、超时
//     - 注意：401/403/404/422/429 等"业务可处理"的 HTTP 状态码统一映射为 200，
//       由 body code 区分。这样能避免被浏览器、代理或网关基于 HTTP 状态码进行
//       自动处理（如 401 自动跳转登录页、429 自动重试）。
//
//  3. 错误码格式：LMMNN 五位（L=Level, MM=Module, NN=具体错误）。
//     Level 1 为系统级（HTTP 500），其他级别均视为业务错误（HTTP 200）。
package errx

import (
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BizCode 是业务错误码类型（格式：LMMNN）。
type BizCode int32

// 错误级别定义。
// Level 决定了 HTTP 响应行为：
//   - LevelSystem：系统级故障，HTTP 5xx
//   - 其他级别：业务可处理的错误，HTTP 200（前端依据 body.code 处理）
const (
	LevelSystem     = 1 // 系统级错误，必须由开发介入（HTTP 500）
	LevelUser       = 2 // 用户操作错误（参数、认证、鉴权等，HTTP 200）
	LevelBusiness   = 3 // 业务逻辑错误（资源不存在、状态冲突等，HTTP 200）
	LevelUpstream   = 4 // 上游服务错误（HTTP 502）
	LevelDownstream = 5 // 下游服务错误（HTTP 502）
)

// 业务模块定义（错误码 LMMNN 中的 MM 段）。
const (
	ModuleCommon   = 0 // 通用模块
	ModuleUser     = 1 // 用户模块
	ModulePost     = 2 // 文章模块
	ModuleComment  = 3 // 评论模块
	ModuleAuth     = 4 // 认证模块
	ModuleDatabase = 5 // 数据库模块
	ModuleCache    = 6 // 缓存模块
)

// 常用错误码定义（格式：LMMNN）。
const (
	CodeOK BizCode = 0 // 成功

	// 用户模块（Level=2 用户级，HTTP 200）
	CodeUserNotFound            BizCode = 20101 // 用户不存在
	CodeUserAlreadyExists       BizCode = 20102 // 用户已存在
	CodeUserInvalidCredentials  BizCode = 20103 // 用户名或密码错误
	CodeUserInsufficientBalance BizCode = 20104 // 用户余额不足
	CodeUserInvalidUsername     BizCode = 20105 // 用户名无效
	CodeUserInvalidPassword     BizCode = 20106 // 密码无效
	CodeUserPermissionDenied    BizCode = 20107 // 用户权限不足

	// 文章模块（Level=3 业务级，HTTP 200）
	CodePostNotFound         BizCode = 30201 // 文章不存在
	CodePostAlreadyPublished BizCode = 30202 // 文章已发布
	CodePostPermissionDenied BizCode = 30203 // 文章权限不足

	// 评论模块（Level=3 业务级，HTTP 200）
	CodeCommentNotFound         BizCode = 30301 // 评论不存在
	CodeCommentPermissionDenied BizCode = 30302 // 评论权限不足

	// 认证模块（Level=2 用户级，HTTP 200）
	CodeAuthUnauthenticated BizCode = 20401 // 未认证
	CodeAuthTokenInvalid    BizCode = 20402 // Token 无效
	CodeAuthTokenExpired    BizCode = 20403 // Token 过期
	CodeAuthSignToken       BizCode = 10404 // Token 签名失败（系统级，HTTP 500）

	// 数据库模块（Level=1 系统级，HTTP 500）
	CodeDatabaseConnectFailed BizCode = 10501 // 数据库连接失败
	CodeDatabaseReadFailed    BizCode = 10502 // 数据库读取失败
	CodeDatabaseWriteFailed   BizCode = 10503 // 数据库写入失败

	// 缓存模块（Level=1 系统级，HTTP 500）
	CodeCacheConnectFailed BizCode = 10601 // 缓存连接失败
	CodeCacheReadFailed    BizCode = 10602 // 缓存读取失败
	CodeCacheWriteFailed   BizCode = 10603 // 缓存写入失败

	// 系统通用错误（Level=1，HTTP 500/502/503/504）
	CodeInternalServer     BizCode = 10001 // 内部服务器错误
	CodeServiceUnavailable BizCode = 10002 // 服务不可用
	CodeRequestTimeout     BizCode = 10003 // 请求超时

	// 参数校验错误（Level=2 用户级，HTTP 200）
	CodeInvalidParameter BizCode = 20001 // 参数无效（绑定失败、缺失等）
	CodeInvalidRequest   BizCode = 20002 // 请求无效（格式错误等）

	// 限流（Level=2 用户级，HTTP 200，前端可提示用户稍后重试）
	CodeTooManyRequests BizCode = 20003
)

// BizError 是业务错误的标准结构。
type BizError struct {
	// Code 业务错误码
	Code BizCode `json:"code"`

	// Level 错误级别（自 Code 派生，便于前端粗略分类）
	Level int `json:"level"`

	// Reason 错误原因（英文，用于日志/监控/告警）
	Reason string `json:"reason,omitempty"`

	// Message 用户友好消息（前端展示）
	Message string `json:"message,omitempty"`

	// Details 详细信息（调试用，可选）
	Details string `json:"details,omitempty"`

	// Metadata 元数据（结构化补充信息）
	Metadata map[string]any `json:"metadata,omitempty"`
}

// APIResponse 是统一的 API 响应结构（前后端分离规范）。
//
// 所有正常到达业务层的请求都会返回此结构，
// HTTP 状态码统一为 200（除系统级故障外）。
type APIResponse struct {
	// Code 0 表示成功，非 0 为业务错误码
	Code int `json:"code"`
	// Message 提示信息（成功用 "success"，失败用业务错误描述）
	Message string `json:"message"`
	// Data 业务数据（仅成功时有值）
	Data any `json:"data,omitempty"`
	// Reason 错误原因（仅失败时有值，便于日志检索）
	Reason string `json:"reason,omitempty"`
	// Details 错误详情（仅失败时有值，便于调试）
	Details string `json:"details,omitempty"`
}

// 常用 HTTP Header。
const (
	HeaderRequestID    = "X-Request-ID"
	HeaderTimestamp    = "X-Timestamp"
	HeaderResponseTime = "X-Response-Time"
	HeaderServerID     = "X-Server-ID"
	HeaderTraceID      = "X-Trace-ID"
)

// GetErrorLevel 从错误码中提取错误级别（最高位）。
func GetErrorLevel(code BizCode) int {
	if code == 0 {
		return 0
	}
	return int(code/10000) % 10
}

// GetModule 从错误码中提取模块编号（中间两位）。
func GetModule(code BizCode) int {
	if code == 0 {
		return 0
	}
	return int(code/100) % 100
}

// GetHTTPCode 根据错误码返回 HTTP 状态码。
//
// 设计原则（前后端分离）：
//   - 任何能正常返回业务响应的情况，统一返回 200
//   - 仅当系统级故障无法工作时，才返回 5xx
//
// 映射规则：
//   - CodeOK         → 200
//   - LevelSystem    → 500（内部错误）
//   - LevelUpstream  → 502（上游网关错误）
//   - LevelDownstream→ 502（下游网关错误）
//   - LevelUser      → 200（用户错误，body code 区分）
//   - LevelBusiness  → 200（业务错误，body code 区分）
//
// 特别注意：未认证 / 鉴权失败 / 资源不存在 / 限流 等场景在本设计中
// 统一返回 200（因为它们已经成功被业务处理并返回了结构化响应），
// 由前端根据 body code 决定是否跳转登录页/重试等。
func GetHTTPCode(code BizCode) int {
	if code == CodeOK {
		return http.StatusOK
	}

	switch GetErrorLevel(code) {
	case LevelSystem:
		// 仅系统级故障返回 5xx
		switch code {
		case CodeServiceUnavailable:
			return http.StatusServiceUnavailable
		case CodeRequestTimeout:
			return http.StatusGatewayTimeout
		default:
			return http.StatusInternalServerError
		}
	case LevelUpstream, LevelDownstream:
		return http.StatusBadGateway
	case LevelUser, LevelBusiness:
		// 业务错误统一返回 200，body 中通过 code 字段区分
		return http.StatusOK
	default:
		// 未明确分类的错误，保守返回 200，避免影响前端处理流程
		return http.StatusOK
	}
}

// NewBizError 创建一个新的业务错误。
func NewBizError(code BizCode, reason, message string) *BizError {
	return &BizError{
		Code:    code,
		Level:   GetErrorLevel(code),
		Reason:  reason,
		Message: message,
	}
}

// WithDetails 在错误上附加详情。
func (err *BizError) WithDetails(details string) *BizError {
	err.Details = details
	return err
}

// WithMetadata 在错误上附加元数据（覆盖式）。
func (err *BizError) WithMetadata(metadata map[string]any) *BizError {
	err.Metadata = metadata
	return err
}

// WithMessage 创建一个新的 BizError（避免共享内存），并替换 Message。
func (err *BizError) WithMessage(message string) *BizError {
	newErr := &BizError{
		Code:    err.Code,
		Level:   err.Level,
		Reason:  err.Reason,
		Message: message,
		Details: err.Details,
	}

	if err.Metadata != nil {
		newErr.Metadata = make(map[string]any, len(err.Metadata))
		for k, v := range err.Metadata {
			newErr.Metadata[k] = v
		}
	}

	return newErr
}

// Error 实现 error 接口。
// 输出包含 details/metadata，便于日志排查；
// 注意：metadata 可能含敏感字段，前端展示请使用 Message。
func (err *BizError) Error() string {
	base := fmt.Sprintf("biz error: code=%d reason=%s message=%s", err.Code, err.Reason, err.Message)
	if err.Details != "" {
		base += fmt.Sprintf(" details=%s", err.Details)
	}
	if len(err.Metadata) > 0 {
		base += fmt.Sprintf(" metadata=%v", err.Metadata)
	}
	return base
}

// Is 判断两个错误是否为相同业务错误（按 code 比较）。
func (err *BizError) Is(target error) bool {
	if bizErr, ok := target.(*BizError); ok {
		return bizErr.Code == err.Code
	}
	return false
}

// ToGRPCCode 将 HTTP 状态码转换为 gRPC 状态码。
func ToGRPCCode(code int) codes.Code {
	switch code {
	case http.StatusOK:
		return codes.OK
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusUnauthorized:
		return codes.Unauthenticated
	case http.StatusForbidden:
		return codes.PermissionDenied
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusConflict:
		return codes.AlreadyExists
	case http.StatusTooManyRequests:
		return codes.ResourceExhausted
	case http.StatusInternalServerError:
		return codes.Internal
	case http.StatusNotImplemented:
		return codes.Unimplemented
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	case http.StatusGatewayTimeout:
		return codes.DeadlineExceeded
	default:
		return codes.Unknown
	}
}

// FromGRPCCode 将 gRPC 状态码转换为 HTTP 状态码。
func FromGRPCCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// Success 创建一个成功响应。
func Success(data any, message ...string) *APIResponse {
	msg := "success"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return &APIResponse{
		Code:    int(CodeOK),
		Message: msg,
		Data:    data,
	}
}

// Failure 创建一个失败响应（不含 reason/details）。
func Failure(code int, message string) *APIResponse {
	return &APIResponse{
		Code:    code,
		Message: message,
		Data:    nil,
	}
}

// WithReason 为响应附加 reason 字段。
func (resp *APIResponse) WithReason(reason string) *APIResponse {
	resp.Reason = reason
	return resp
}

// WithDetails 为响应附加 details 字段。
func (resp *APIResponse) WithDetails(details string) *APIResponse {
	resp.Details = details
	return resp
}

// FromBizError 把业务错误转换为统一的 API 响应。
func FromBizError(err *BizError) *APIResponse {
	resp := Failure(int(err.Code), err.Message)
	resp.Reason = err.Reason
	resp.Details = err.Details
	return resp
}

// GRPCStatus 返回该错误对应的 gRPC 状态。
func (err *BizError) GRPCStatus() *status.Status {
	details := errdetails.ErrorInfo{Reason: err.Reason}
	if err.Metadata != nil {
		details.Metadata = make(map[string]string, len(err.Metadata))
		for k, v := range err.Metadata {
			details.Metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	s, _ := status.New(ToGRPCCode(GetHTTPCode(err.Code)), err.Message).WithDetails(&details)
	return s
}

// Code 从 error 中提取业务错误码。
// 非 BizError 类型默认返回 CodeInternalServer。
func Code(err error) int {
	if err == nil {
		return int(CodeOK)
	}
	if bizErr := new(BizError); errors.As(err, &bizErr) {
		return int(bizErr.Code)
	}
	return int(CodeInternalServer)
}

// Reason 从 error 中提取业务错误原因。
func Reason(err error) string {
	if err == nil {
		return ""
	}
	if bizErr := new(BizError); errors.As(err, &bizErr) {
		return bizErr.Reason
	}
	return "InternalError"
}

// FromError 把任意 error 规范化为 *BizError。
//
// 处理顺序：
//  1. 已经是 *BizError 直接返回
//  2. gRPC status：根据 code 推断 BizCode
//  3. 兜底：包装为 CodeInternalServer
func FromError(err error) *BizError {
	if err == nil {
		return nil
	}

	if bizErr := new(BizError); errors.As(err, &bizErr) {
		return bizErr
	}

	gs, ok := status.FromError(err)
	if ok {
		bizCode := CodeInternalServer
		switch FromGRPCCode(gs.Code()) {
		case http.StatusBadRequest:
			bizCode = CodeInvalidParameter
		case http.StatusUnauthorized:
			bizCode = CodeAuthUnauthenticated
		case http.StatusForbidden:
			bizCode = CodeUserPermissionDenied
		case http.StatusNotFound:
			bizCode = CodeUserNotFound
		case http.StatusTooManyRequests:
			bizCode = CodeTooManyRequests
		}

		bizErr := NewBizError(bizCode, "gRPC", gs.Message())

		for _, detail := range gs.Details() {
			if typed, ok := detail.(*errdetails.ErrorInfo); ok {
				bizErr.Reason = typed.Reason
				if len(typed.Metadata) > 0 {
					metadata := make(map[string]any, len(typed.Metadata))
					for k, v := range typed.Metadata {
						metadata[k] = v
					}
					bizErr.Metadata = metadata
				}
				break
			}
		}

		return bizErr
	}

	return NewBizError(CodeInternalServer, "Unknown", err.Error())
}
