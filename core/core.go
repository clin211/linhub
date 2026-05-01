package core

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/clin211/linhub/binding"
	"github.com/clin211/linhub/errx"
)

// Validator 是验证函数的类型，用于对绑定的数据结构进行验证.
type Validator[T any] func(context.Context, *T) error

// Binder 定义绑定函数的类型，用于绑定请求数据到相应结构体.
type Binder func(any) error

// Handler 是处理函数的类型，用于处理已经绑定和验证的数据.
type Handler[T any, R any] func(ctx context.Context, req *T) (R, error)

// generateRequestID 生成请求ID
func generateRequestID() string {
	return uuid.New().String()
}

var (
	serverIDOnce sync.Once
	serverID     string
)

// getServerID 获取服务器ID（环境变量 SERVER_ID，进程启动后只读取一次并缓存）。
func getServerID() string {
	serverIDOnce.Do(func() {
		serverID = os.Getenv("SERVER_ID")
	})
	return serverID
}

// setResponseHeaders 设置响应头
func setResponseHeaders(c *gin.Context, requestID string, startTime time.Time) {
	c.Header(errx.HeaderRequestID, requestID)

	c.Header(errx.HeaderTimestamp, strconv.FormatInt(time.Now().Unix(), 10))

	responseTime := time.Since(startTime).Milliseconds()
	c.Header(errx.HeaderResponseTime, strconv.FormatInt(responseTime, 10))

	if serverID := getServerID(); serverID != "" {
		c.Header(errx.HeaderServerID, serverID)
	}

	if traceID, exists := c.Get("trace_id"); exists {
		c.Header(errx.HeaderTraceID, traceID.(string))
	}
}

// ResponseMiddleware 统一响应格式中间件
func ResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		requestID := c.GetHeader(errx.HeaderRequestID)
		if requestID == "" {
			requestID = generateRequestID()
		}

		// 存储到 context 供后续使用
		c.Set("request_id", requestID)

		c.Next()

		// 设置响应头
		setResponseHeaders(c, requestID, startTime)
	}
}

// HandleAllRequest 是处理综合请求的快捷函数。
// 它会将 URI 参数、Query/Form 参数、以及 JSON Body 一并绑定到结构体中，
// 然后可选执行多个验证器，最后调用业务逻辑处理函数。
func HandleAllRequest[T any, R any](c *gin.Context, handler Handler[T, R], validators ...Validator[T]) {
	var request T

	// 绑定和验证请求数据
	if err := ShouldBindAll(c, &request, validators...); err != nil {
		WriteResponse(c, nil, err)
		return
	}

	// 调用实际的业务逻辑处理函数
	response, err := handler(c.Request.Context(), &request)
	WriteResponse(c, response, err)
}

// HandleJSONRequest 是处理 JSON 请求的快捷函数.
func HandleJSONRequest[T any, R any](c *gin.Context, handler Handler[T, R], validators ...Validator[T]) {
	HandleRequest(c, c.ShouldBindJSON, handler, validators...)
}

// HandleQueryRequest 是处理 Query 参数请求的快捷函数.
func HandleQueryRequest[T any, R any](c *gin.Context, handler Handler[T, R], validators ...Validator[T]) {
	HandleRequest(c, c.ShouldBindQuery, handler, validators...)
}

// HandleUriRequest 是处理 URI 请求的快捷函数.
func HandleUriRequest[T any, R any](c *gin.Context, handler Handler[T, R], validators ...Validator[T]) {
	HandleRequest(c, c.ShouldBindUri, handler, validators...)
}

// HandleRequest 是通用的请求处理函数.
// 负责绑定请求数据、执行验证、并调用实际的业务处理逻辑函数.
func HandleRequest[T any, R any](c *gin.Context, binder Binder, handler Handler[T, R], validators ...Validator[T]) {
	var request T

	// 绑定和验证请求数据
	if err := ReadRequest(c, &request, binder, validators...); err != nil {
		WriteResponse(c, nil, err)
		return
	}

	// 调用实际的业务逻辑处理函数
	response, err := handler(c.Request.Context(), &request)
	WriteResponse(c, response, err)
}

// ShouldBindJSON 使用 JSON 格式的绑定函数绑定请求参数并执行验证。
func ShouldBindJSON[T any](c *gin.Context, rq *T, validators ...Validator[T]) error {
	return ReadRequest(c, rq, c.ShouldBindJSON, validators...)
}

// ShouldBindQuery 使用 Query 格式的绑定函数绑定请求参数并执行验证。
func ShouldBindQuery[T any](c *gin.Context, rq *T, validators ...Validator[T]) error {
	return ReadRequest(c, rq, c.ShouldBindQuery, validators...)
}

// ShouldBindUri 使用 URI 格式的绑定函数绑定请求参数并执行验证。
func ShouldBindUri[T any](c *gin.Context, rq *T, validators ...Validator[T]) error {
	return ReadRequest(c, rq, c.ShouldBindUri, validators...)
}

// ShouldBindAll 将 URI 参数、Query/Form 参数以及 JSON Body 一并绑定到结构体中。
// 它能覆盖同名字段（后者优先），并支持 Default() 与验证函数 validators。
func ShouldBindAll[T any](c *gin.Context, rq *T, validators ...Validator[T]) error {
	if err := binding.Bind(c, rq, binding.URI, binding.JSON); err != nil {
		return errx.ErrBind.WithDetails(err.Error())
	}

	if err := FinalizeRequest(c, rq, validators...); err != nil {
		return err
	}

	return nil
}

// ReadRequest 是用于绑定和验证请求数据的通用工具函数.
// - 它负责调用绑定函数绑定请求数据.
// - 如果目标类型实现了 Default 接口，会调用其 Default 方法设置默认值.
// - 最后执行传入的验证器对数据进行校验.
func ReadRequest[T any](c *gin.Context, rq *T, binder Binder, validators ...Validator[T]) error {
	if err := binder(rq); err != nil {
		return errx.ErrBind.WithDetails(err.Error())
	}

	if err := FinalizeRequest(c, rq, validators...); err != nil {
		return err
	}

	return nil
}

// FinalizeRequest 在请求参数绑定完成后执行以下操作：
// 1. 如果目标类型实现了 Default() 方法，则调用 Default 设置默认值；
// 2. 顺序执行所有验证函数。
func FinalizeRequest[T any](c *gin.Context, rq *T, validators ...Validator[T]) error {
	// 应用默认值
	if defaulter, ok := any(rq).(interface{ Default() }); ok {
		defaulter.Default()
	}

	// 执行验证逻辑
	for _, validate := range validators {
		if validate == nil {
			continue
		}
		if err := validate(c.Request.Context(), rq); err != nil {
			return err
		}
	}

	return nil
}

// WriteResponse 是通用的响应函数。
//
// 设计原则（前后端分离）：
//   - 业务错误统一返回 HTTP 200，body 中通过 code 区分（详见 errx.GetHTTPCode）
//   - 仅当系统级故障（数据库不可用、超时、内部错误等）时返回 5xx
//   - 前端只需根据 body.code 判断业务结果，无需依赖 HTTP 状态码语义
func WriteResponse(c *gin.Context, data any, err error) {
	if err != nil {
		bizErr := errx.FromError(err)
		response := errx.FromBizError(bizErr)

		httpCode := errx.GetHTTPCode(bizErr.Code)
		c.JSON(httpCode, response)
		return
	}

	response := errx.Success(data)
	c.JSON(http.StatusOK, response)
}

// WriteBizError 写入业务错误的便捷函数。
func WriteBizError(c *gin.Context, bizErr *errx.BizError) {
	response := errx.FromBizError(bizErr)
	httpCode := errx.GetHTTPCode(bizErr.Code)
	c.JSON(httpCode, response)
}

// WriteSuccess 写入成功响应的便捷函数。
func WriteSuccess(c *gin.Context, data any, message ...string) {
	response := errx.Success(data, message...)
	c.JSON(http.StatusOK, response)
}
