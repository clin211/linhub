package errx

// 预定义的标准错误。
//
// 为方便业务代码复用，本文件提供常用错误的预制实例。
// 业务可在此基础上通过 WithMessage / WithDetails / WithMetadata 衍生新错误。
var (
	// OK 表示请求成功（业务码为 0）。
	OK = NewBizError(CodeOK, "OK", "success")

	// ErrInternal 表示所有未知的服务器端错误（HTTP 500）。
	ErrInternal = NewBizError(CodeInternalServer, "InternalError", "Internal server error.")

	// ErrServiceUnavailable 表示服务暂不可用（HTTP 503）。
	ErrServiceUnavailable = NewBizError(CodeServiceUnavailable, "ServiceUnavailable", "Service unavailable.")

	// ErrRequestTimeout 表示请求超时（HTTP 504）。
	ErrRequestTimeout = NewBizError(CodeRequestTimeout, "RequestTimeout", "Request timeout.")

	// ErrTooManyRequests 表示请求过于频繁（业务级 → HTTP 200）。
	ErrTooManyRequests = NewBizError(CodeTooManyRequests, "TooManyRequests", "Too many requests, please try again later.")

	// ErrNotFound 表示请求的资源不存在（HTTP 200）。
	ErrNotFound = NewBizError(CodeUserNotFound, "NotFound", "Resource not found.")

	// ErrBind 表示请求体绑定失败（HTTP 200）。
	ErrBind = NewBizError(CodeInvalidParameter, "BindError", "Failed to bind request body.")

	// ErrInvalidArgument 表示参数校验失败（HTTP 200）。
	ErrInvalidArgument = NewBizError(CodeInvalidParameter, "InvalidArgument", "Invalid argument.")

	// ErrInvalidRequest 表示请求格式无效（HTTP 200）。
	ErrInvalidRequest = NewBizError(CodeInvalidRequest, "InvalidRequest", "Invalid request.")

	// ErrUnauthenticated 表示用户未认证（HTTP 200，前端需根据 code 处理登录跳转）。
	ErrUnauthenticated = NewBizError(CodeAuthUnauthenticated, "Unauthenticated", "Unauthenticated.")

	// ErrTokenInvalid 表示 token 无效（HTTP 200）。
	ErrTokenInvalid = NewBizError(CodeAuthTokenInvalid, "TokenInvalid", "Token is invalid.")

	// ErrTokenExpired 表示 token 已过期（HTTP 200）。
	ErrTokenExpired = NewBizError(CodeAuthTokenExpired, "TokenExpired", "Token has expired.")

	// ErrPermissionDenied 表示用户无权限访问该资源（HTTP 200）。
	ErrPermissionDenied = NewBizError(CodeUserPermissionDenied, "PermissionDenied", "Permission denied.")

	// ErrOperationFailed 表示请求的操作失败（HTTP 500）。
	ErrOperationFailed = NewBizError(CodeInternalServer, "OperationFailed", "Operation failed, please try again later.")
)
