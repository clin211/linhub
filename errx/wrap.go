package errx

import "errors"

// Is 报告 err 链中是否存在任何错误与 target 匹配。
func Is(err, target error) bool { return errors.Is(err, target) }

// As 在 err 链中查找第一个与 target 匹配的错误，
// 若找到则将 target 设置为该错误值并返回 true。
func As(err error, target any) bool { return errors.As(err, target) }

// Unwrap 返回对 err 调用 Unwrap 方法的结果。
func Unwrap(err error) error { return errors.Unwrap(err) }
