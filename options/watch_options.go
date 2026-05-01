package options

import (
	"errors"
	"time"

	"github.com/spf13/pflag"
)

// 确保实现了 IOptions 接口
var _ IOptions = (*WatchOptions)(nil)

// WatchOptions 结构体保存创建并运行 watch 服务器所需的配置选项。
type WatchOptions struct {
	// LockName 指定服务器使用的锁的名称。
	LockName string `json:"lock-name" mapstructure:"lock-name"`

	// HealthzPort 是健康检查端点的端口号。
	HealthzPort int `json:"healthz-port" mapstructure:"healthz-port"`

	// DisableWatchers 是服务器运行时将被禁用的 watcher 列表。
	DisableWatchers []string `json:"disable-watchers" mapstructure:"disable-watchers"`

	// MaxWorkers 定义每个 watcher 可以创建的最大并发 worker 数量。
	MaxWorkers int64 `json:"max-workers" mapstructure:"max-workers"`

	// WatchTimeout 定义单个 watch 执行的超时时长。
	WatchTimeout time.Duration `json:"watch-timeout" mapstructure:"watch-timeout"`

	// PerConcurrency 定义每个独立 watcher 允许的最大并发执行数量。
	PerConcurrency int `json:"per-watch-concurrency" mapstructure:"per-watch-concurrency"`
}

// NewWatchOptions 初始化并返回一个使用默认值的新 WatchOptions 实例。
func NewWatchOptions() *WatchOptions {
	o := &WatchOptions{
		LockName:        "default-distributed-watch-lock",
		HealthzPort:     8881,
		DisableWatchers: []string{},
		MaxWorkers:      1000,
		WatchTimeout:    30 * time.Second,
		PerConcurrency:  10,
	}

	return o
}

// AddFlags 将与 WatchOptions 结构体相关的命令行标志添加到提供的 FlagSet。
// 这将允许用户通过命令行参数来配置 watch 服务器。
func (o *WatchOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.LockName, fullPrefix+".lock-name", o.LockName,
		"服务器使用的锁的名称。")

	fs.IntVar(&o.HealthzPort, fullPrefix+".healthz-port", o.HealthzPort,
		"健康检查端点的端口号。")

	fs.StringSliceVar(&o.DisableWatchers, fullPrefix+".disable-watchers", o.DisableWatchers,
		"应该被禁用的 watcher 列表。")

	fs.Int64Var(&o.MaxWorkers, fullPrefix+".max-workers", o.MaxWorkers,
		"指定每个 watcher 的最大并发 worker 数量。")

	fs.DurationVar(&o.WatchTimeout, fullPrefix+".timeout", o.WatchTimeout,
		"单个 watch 执行的超时时长（例如：30s、2m、1h）。")

	fs.IntVar(&o.PerConcurrency, fullPrefix+".per-concurrency", o.PerConcurrency,
		"每个独立 watcher 允许的最大并发执行数量。")
}

// Validate 检查 WatchOptions 结构体中的必要配置，并返回错误切片。
func (o *WatchOptions) Validate() []error {
	errs := []error{}

	// 校验 LockName
	if o.LockName == "" {
		errs = append(errs, errors.New("lock-name cannot be empty"))
	}

	// 校验 HealthzPort
	if o.HealthzPort <= 0 || o.HealthzPort > 65535 {
		errs = append(errs, errors.New("healthz-port must be between 1 and 65535"))
	}

	// 校验 MaxWorkers
	if o.MaxWorkers <= 0 {
		errs = append(errs, errors.New("max-workers must be greater than 0"))
	}

	// 校验 WatchTimeout
	if o.WatchTimeout <= 0 {
		errs = append(errs, errors.New("watch-timeout must be greater than 0"))
	}

	// 检查合理的超时上限（可选但推荐）
	if o.WatchTimeout > 24*time.Hour {
		errs = append(errs, errors.New("watch-timeout should not exceed 24 hours for practical reasons"))
	}

	// 校验 PerConcurrency
	if o.PerConcurrency <= 0 {
		errs = append(errs, errors.New("per-watch-concurrency must be greater than 0"))
	}

	// 检查合理的并发上限
	if o.PerConcurrency > 1000 {
		errs = append(errs, errors.New("per-watch-concurrency should not exceed 1000 for resource management"))
	}

	// 交叉校验：PerConcurrency 不应超过 MaxWorkers
	if int64(o.PerConcurrency) > o.MaxWorkers {
		errs = append(errs, errors.New("per-watch-concurrency cannot be greater than max-workers"))
	}

	// 校验 DisableWatchers（可选：检查重复项）
	watcherSet := make(map[string]bool)
	for _, watcher := range o.DisableWatchers {
		if watcher == "" {
			errs = append(errs, errors.New("disable-watchers cannot contain empty strings"))
			continue
		}
		if watcherSet[watcher] {
			errs = append(errs, errors.New("disable-watchers contains duplicate entries: "+watcher))
		}
		watcherSet[watcher] = true
	}

	return errs
}
