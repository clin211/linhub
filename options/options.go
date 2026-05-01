package options

import "github.com/spf13/pflag"

// IOptions 定义实现通用选项的方法。
type IOptions interface {
	// Validate 校验所有必需的选项。
	// 必要时也可用于补全选项。
	Validate() []error

	// AddFlags 将所有选项字段注册为命令行标志到给定的 FlagSet 上，
	// 直接使用提供的 fullPrefix。
	//
	// fullPrefix 应当是一个完整的前缀字符串，例如："onex.otel"。
	// 实现需要将自身字段名附加到该前缀上以构建最终的标志名，例如：
	//   --onex.otel.endpoint
	//   --onex.otel.insecure
	AddFlags(fs *pflag.FlagSet, fullPrefix string)
}
