package app

import (
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
)

// OptionsValidator 提供补全和校验选项的方法。
// 任何需要选项校验的组件都应实现此接口。
type OptionsValidator interface {
	// Complete 补全所有必需的选项。
	Complete() error

	// Validate 校验所有必需的选项。
	Validate() error
}

// NamedFlagSetOptions 提供对特定服务器命名标志集的访问，并嵌入了校验功能。
type NamedFlagSetOptions interface {
	// Flags 按 section 名称返回特定服务器的标志集。
	Flags() cliflag.NamedFlagSets

	OptionsValidator
}

// FlagSetOptions 定义命令行选项的接口，
// 这些选项可以将自身添加到一个标志集中并执行校验。
type FlagSetOptions interface {
	// AddFlags 将命令特定的标志添加到提供的标志集中。
	AddFlags(fs *pflag.FlagSet)

	OptionsValidator
}
