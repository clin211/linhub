package ptr

import (
	"fmt"
	"reflect"
)

// AllPtrFieldsNil 测试结构体中所有的指针字段是否都为 nil。该函数在以下场景中很有用：
// 例如，当一个 API 结构体由插件处理时，需要区分
// "没有插件接受此规范" 和 "此规范为空" 这两种情况。
//
// 此函数仅适用于结构体和指向结构体的指针。任何其他
// 类型都会导致 panic。传入一个带类型的 nil 指针会返回 true。
func AllPtrFieldsNil(obj interface{}) bool {
	v := reflect.ValueOf(obj)
	if !v.IsValid() {
		panic(fmt.Sprintf("reflect.ValueOf() produced a non-valid Value for %#v", obj))
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.Ptr && !v.Field(i).IsNil() {
			return false
		}
	}
	return true
}

// To 返回指向给定值的指针。
func To[T any](v T) *T {
	return &v
}

// From 返回指针 p 指向的值。
// 如果指针为 nil，则返回 T 的零值。
func From[T any](v *T) T {
	var zero T
	if v != nil {
		return *v
	}

	return zero
}

// FromOr 解引用 ptr 并返回它指向的值（如果非 nil），否则
// 返回 def。
func FromOr[T any](ptr *T, def T) T {
	if ptr != nil {
		return *ptr
	}
	return def
}

// IsNil 返回给定指针 v 是否为 nil。
func IsNil[T any](p *T) bool {
	return p == nil
}

// IsNotNil 是 [IsNil] 的取反。
func IsNotNil[T any](p *T) bool {
	return p != nil
}

// Clone 返回切片的浅拷贝。
// 如果给定的指针为 nil，则返回 nil。
//
// 提示：元素是通过赋值（=）方式复制的，因此这是一个浅拷贝。
// 如果你想进行深拷贝，请使用 [CloneBy] 并提供合适的元素
// 克隆函数。
//
// 别名：Copy
func Clone[T any](p *T) *T {
	if p == nil {
		return nil
	}
	clone := *p
	return &clone
}

// CloneBy 是 [Clone] 的变体，它返回 map 的副本。
// 元素通过函数 f 进行复制。
// 如果给定的指针为 nil，则返回 nil。
func CloneBy[T any](p *T, f func(T) T) *T {
	return Map(p, f)
}

// Equal 当两个参数都为 nil 或两个参数
// 解引用后的值相等时，返回 true。
func Equal[T comparable](a, b *T) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

// EqualTo 返回指针 p 的值是否等于值 v。
// 它是 "x != nil && *x == y" 的简写形式。
//
// 示例：
//
//	x, y := 1, 2
//	Equal(&x, 1)   ⏩  true
//	Equal(&y, 1)   ⏩n false
//	Equal(nil, 1)  ⏩  false
func EqualTo[T comparable](p *T, v T) bool {
	return p != nil && *p == v
}

// Map 将函数 f 应用于指针 p 指向的元素。
// 如果 p 为 nil，则不会调用 f 并返回 nil；否则
// f 的执行结果将作为一个新指针返回。
//
// 示例：
//
//	i := 1
//	Map(&i, strconv.Itoa)       ⏩  (*string)("1")
//	Map[int](nil, strconv.Itoa) ⏩  (*string)(nil)
func Map[F, T any](p *F, f func(F) T) *T {
	if p == nil {
		return nil
	}
	return To(f(*p))
}
