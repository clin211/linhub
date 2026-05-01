package binding

import (
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// 包内部变量
var (
	// originalValidator 存储原始的 Gin 校验器，用于最终的统一校验
	originalValidator binding.StructValidator

	// initOnce 确保校验器只被捕获一次
	initOnce sync.Once
)

// init 捕获原始的校验器并将全局校验器设置为 nil。
// 此操作在包导入时执行，且仅执行一次。
func init() {
	// 使用 Once 确保即使多个 goroutine 同时进入也只执行一次
	initOnce.Do(func() {
		// 保存原始校验器
		originalValidator = binding.Validator

		// 将全局校验器设置为 nil，从而在绑定过程中跳过校验
		binding.Validator = nil
	})
}

// Bind 在多个数据源之间逐个绑定数据，过程中不执行校验，
// 待全部绑定完成后再统一执行一次校验。
//
// 这解决了从单一数据源（例如 URI）绑定时因为
// 其他数据源（例如 JSON）的字段尚未绑定而导致校验失败的问题。
//
// 使用示例：
//
//	var req UserUpdateRequest
//	if err := binding.Bind(c, &req, binding.URI, binding.JSON); err != nil {
//	    c.JSON(http.StatusBadRequest, ErrorResponse{Message: err.Error()})
//	    return
//	}
func Bind(c *gin.Context, obj interface{}, bindFuncs ...func(*gin.Context, interface{}) error) error {
	// 执行所有绑定函数（此时校验已被禁用）
	for _, bindFunc := range bindFuncs {
		if err := bindFunc(c, obj); err != nil {
			return err // 返回解析错误，但不返回校验错误
		}
	}

	// 所有绑定完成后再手动执行一次校验
	if originalValidator != nil {
		return originalValidator.ValidateStruct(obj)
	}

	return nil
}

// 用于 Bind 的常用绑定函数

// URI 将 URI 参数绑定到给定对象。
// 使用 Gin 的 ShouldBindUri，但不执行校验。
func URI(c *gin.Context, obj interface{}) error {
	return c.ShouldBindUri(obj)
}

// JSON 将 JSON 请求体绑定到给定对象。
// 使用 Gin 的 ShouldBindJSON，但不执行校验。
func JSON(c *gin.Context, obj interface{}) error {
	return c.ShouldBindJSON(obj)
}

// Query 将 URL Query 参数绑定到给定对象。
func Query(c *gin.Context, obj interface{}) error {
	return c.ShouldBindQuery(obj)
}

// Form 将 application/x-www-form-urlencoded 表单绑定到给定对象。
func Form(c *gin.Context, obj interface{}) error {
	return c.ShouldBind(obj)
}

// MultipartForm 将 multipart/form-data 表单绑定到给定对象。
func MultipartForm(c *gin.Context, obj interface{}) error {
	return c.ShouldBindWith(obj, binding.FormMultipart)
}

// Header 将请求头绑定到给定对象。
func Header(c *gin.Context, obj interface{}) error {
	return c.ShouldBindHeader(obj)
}
