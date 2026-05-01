package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

const configFlagName = "config"

var cfgFile string

// AddConfigFlag 为指定的服务器向给定的 FlagSet 对象添加标志。
// 它还设置了当每个 cobra 命令的 Execute 方法被调用时，
// 从配置文件读取值到 viper 中的相关函数。
func AddConfigFlag(fs *pflag.FlagSet, name string, watch bool) {
	fs.AddFlag(pflag.Lookup(configFlagName))

	// 启用 viper 的自动环境变量解析。这意味着 viper 将自动从
	// 环境变量中读取对应于 viper 变量的值。
	viper.AutomaticEnv()
	// 设置环境变量前缀。使用 strings.ReplaceAll 函数将 name 中的连字符
	// 替换为下划线，并使用 strings.ToUpper 将 name 转换为大写，
	// 然后将其设置为环境变量的前缀。
	viper.SetEnvPrefix(strings.ReplaceAll(strings.ToUpper(name), "-", "_"))
	// 设置环境变量键的替换规则。使用 strings.NewReplacer 函数
	// 指定将句点和连字符替换为下划线。
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	cobra.OnInitialize(func() {
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
		} else {
			viper.AddConfigPath(".")

			if names := strings.Split(name, "-"); len(names) > 1 {
				viper.AddConfigPath(filepath.Join(homedir.HomeDir(), "."+names[0]))
				viper.AddConfigPath(filepath.Join("/etc", names[0]))
			}

			viper.SetConfigName(name)
		}

		if err := viper.ReadInConfig(); err != nil {
			klog.V(2).InfoS("Failed to read configuration file", "file", cfgFile, "err", err)
		}
		klog.V(2).InfoS("Success to read configuration file", "file", viper.ConfigFileUsed())

		if watch {
			viper.WatchConfig()
			viper.OnConfigChange(func(e fsnotify.Event) {
				klog.V(2).InfoS("Config file changed", "name", e.Name)
			})
		}
	})
}

func PrintConfig() {
	for _, key := range viper.AllKeys() {
		klog.V(2).InfoS(fmt.Sprintf("CFG: %s=%v", key, viper.Get(key)))
	}
}

func init() {
	pflag.StringVarP(&cfgFile, configFlagName, "c", cfgFile, "Read configuration from specified `FILE`, "+
		"support JSON, TOML, YAML, HCL, or Java properties formats.")
}
