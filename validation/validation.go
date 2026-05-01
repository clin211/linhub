package validation

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/klog/v2"
)

// Validator 实现了 validate.IValidator 接口。
type Validator struct {
	registry map[string]reflect.Value
}

// ProviderSet 是 validator 的提供者集合。
//var ProviderSet = wire.NewSet(NewValidator)

// NewValidator 创建并初始化一个自定义验证器。
func NewValidator(customValidator any) *Validator {
	return &Validator{registry: extractValidationMethods(customValidator)}
}

// Validate 使用合适的验证方法验证请求。
func (v *Validator) Validate(ctx context.Context, request any) error {
	validationFunc, ok := v.registry[reflect.TypeOf(request).Elem().Name()]
	if !ok {
		return nil // 未找到该请求类型的验证函数
	}

	result := validationFunc.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(request)})
	if !result[0].IsNil() {
		return result[0].Interface().(error)
	}

	return nil
}

// extractValidationMethods 从提供的自定义验证器中提取并返回验证函数的 map。
func extractValidationMethods(customValidator any) map[string]reflect.Value {
	funcs := make(map[string]reflect.Value)
	validatorType := reflect.TypeOf(customValidator)
	validatorValue := reflect.ValueOf(customValidator)

	for i := 0; i < validatorType.NumMethod(); i++ {
		method := validatorType.Method(i)
		methodValue := validatorValue.MethodByName(method.Name)

		if !methodValue.IsValid() || !strings.HasPrefix(method.Name, "Validate") {
			continue
		}

		methodType := methodValue.Type()

		// 确保方法接收一个 context.Context 和一个指针
		if methodType.NumIn() != 2 || methodType.NumOut() != 1 ||
			methodType.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() ||
			methodType.In(1).Kind() != reflect.Pointer {
			continue
		}

		// 确保方法名符合预期的命名约定
		requestTypeName := methodType.In(1).Elem().Name()
		if method.Name != ("Validate" + requestTypeName) {
			continue
		}

		// 确保返回类型为 error
		if methodType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}

		klog.V(4).InfoS("Registering validator", "validator", requestTypeName)
		funcs[requestTypeName] = methodValue
	}

	return funcs
}

// ValidRequired 验证结构体中的必需字段是否存在且不为空.
func ValidRequired(obj any, requiredFields ...string) error {
	val := reflect.ValueOf(obj)

	// 检查 obj 是否为结构体或指向结构体的指针
	if val.Kind() != reflect.Struct && val.Kind() != reflect.Ptr {
		return fmt.Errorf("input must be a struct or a pointer to a struct. Got %s", val.Kind().String())
	}

	// 如果是指针，获取指向的结构体值
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 仍需确保最终值是结构体
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("input must be a struct or a pointer to a struct. Got %s", val.Kind().String())
	}

	// 遍历需要验证的字段
	for _, field := range requiredFields {
		// 使用反射获取字段
		fieldVal := val.FieldByName(field)

		// 判断字段是否存在
		if !fieldVal.IsValid() {
			return fmt.Errorf("field %s does not exist in struct", field)
		}

		// 检查字段是否为 nil
		if fieldVal.IsNil() {
			return fmt.Errorf("field %s must be provided", field)
		}
	}

	return nil
}
