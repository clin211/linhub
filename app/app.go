package app

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
	"k8s.io/component-base/cli"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/term"
	"k8s.io/klog/v2"

	"github.com/clin211/linhub/log"
	genericoptions "github.com/clin211/linhub/options"
	"github.com/clin211/linhub/version"
)

// App 是命令行应用程序的主要结构。
// 推荐使用 app.NewApp() 函数来创建一个应用程序。
type App struct {
	name        string
	shortDesc   string
	description string
	run         RunFunc
	runCtx      RunContextFunc
	cmd         *cobra.Command
	args        cobra.PositionalArgs

	// +optional
	healthCheckFunc HealthCheckFunc

	// +optional
	options any

	// +optional
	silence bool

	// +optional
	noConfig bool

	// 监视并重新读取配置文件
	// +optional
	watch bool

	contextExtractors map[string]func(context.Context) string
}

// RunFunc 定义应用程序的启动回调函数（兼容旧 API）。
type RunFunc func() error

// RunContextFunc 定义带 context 的启动回调函数。
// ctx 在收到 SIGINT/SIGTERM 时会被取消，业务代码可以监听 ctx.Done() 实现优雅退出。
type RunContextFunc func(ctx context.Context) error

// HealthCheckFunc 定义应用程序的健康检查函数。
type HealthCheckFunc func() error

// Option 定义初始化应用程序结构的可选参数。
type Option func(*App)

// WithOptions 开启应用程序从命令行或配置文件读取参数的功能。
func WithOptions(opts any) Option {
	return func(app *App) {
		app.options = opts
	}
}

// WithRunFunc 用于设置应用程序的启动回调函数选项（兼容旧 API）。
func WithRunFunc(run RunFunc) Option {
	return func(app *App) {
		app.run = run
	}
}

// WithContextRunFunc 用于设置带 context 的启动回调函数。
// 与 WithRunFunc 互斥；若同时设置，优先使用 RunContextFunc。
// ctx 会在 SIGINT/SIGTERM 时被取消，业务可监听以实现优雅退出。
func WithContextRunFunc(run RunContextFunc) Option {
	return func(app *App) {
		app.runCtx = run
	}
}

// WithDescription 用于设置应用程序的描述信息。
func WithDescription(desc string) Option {
	return func(app *App) {
		app.description = desc
	}
}

// WithHealthCheckFunc 用于设置应用程序的健康检查函数。
// app 框架将使用该函数启动一个健康检查服务器。
func WithHealthCheckFunc(fn HealthCheckFunc) Option {
	return func(app *App) {
		app.healthCheckFunc = fn
	}
}

// WithDefaultHealthCheckFunc 设置默认的健康检查函数。
// 健康检查服务在独立 goroutine 中启动，监听失败仅记录日志，不会终止主进程。
func WithDefaultHealthCheckFunc() Option {
	fn := func() HealthCheckFunc {
		return func() error {
			go func() {
				if err := genericoptions.NewHealthOptions().ServeHealthCheck(); err != nil {
					klog.ErrorS(err, "Health check server exited with error")
				}
			}()
			return nil
		}
	}

	return WithHealthCheckFunc(fn())
}

// WithSilence 将应用程序设置为静默模式，在该模式下，
// 程序启动信息、配置信息和版本信息不会在控制台打印。
func WithSilence() Option {
	return func(app *App) {
		app.silence = true
	}
}

// WithNoConfig 设置应用程序不提供 config 标志。
func WithNoConfig() Option {
	return func(app *App) {
		app.noConfig = true
	}
}

// WithValidArgs 设置用于验证非标志参数的校验函数。
func WithValidArgs(args cobra.PositionalArgs) Option {
	return func(app *App) {
		app.args = args
	}
}

// WithDefaultValidArgs 设置用于验证非标志参数的默认校验函数。
func WithDefaultValidArgs() Option {
	return func(app *App) {
		app.args = cobra.NoArgs
	}
}

// WithWatchConfig 监视并重新读取配置文件。
func WithWatchConfig() Option {
	return func(app *App) {
		app.watch = true
	}
}

func WithLoggerContextExtractor(contextExtractors map[string]func(context.Context) string) Option {
	return func(app *App) {
		app.contextExtractors = contextExtractors
	}
}

// NewApp 根据给定的应用程序名称、二进制名称及其他选项创建一个新的应用程序实例。
func NewApp(name string, shortDesc string, opts ...Option) *App {
	app := &App{
		name:      name,
		run:       func() error { return nil },
		shortDesc: shortDesc,
	}

	for _, o := range opts {
		o(app)
	}

	app.buildCommand()

	return app
}

// buildCommand 用于构建一个 cobra 命令。
func (app *App) buildCommand() {
	cmd := &cobra.Command{
		Use:   formatBaseName(app.name),
		Short: app.shortDesc,
		Long:  app.description,
		RunE:  app.runCommand,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return nil
		},
		Args: app.args,
	}
	// 当 Cobra 命令启用错误打印时，会先打印一个标志解析
	// 错误，然后可选地打印通常很长的使用说明文本。这在
	// 控制台中非常难以阅读，因为屏幕上可见的最后几行
	// 并不包含错误信息。
	//
	// #sig-cli 的建议是先打印使用说明文本，再打印错误信息。
	// 我们在此处统一为所有命令实现该行为。然而，当命令
	// 因解析以外的其他原因执行失败时，我们不想打印使用说明
	// 文本。我们通过 FlagParseError 回调来检测这种情况。
	//
	// 一些命令（如 kubectl）已经自行处理了这个问题，
	// 我们不会改变它们的行为。
	if !cmd.SilenceUsage {
		cmd.SilenceUsage = true
		cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
			// 重新启用使用说明打印。
			c.SilenceUsage = false
			return err
		})
	}
	// 在所有情况下，错误打印都在下方完成。
	cmd.SilenceErrors = true

	cmd.SetOutput(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.Flags().SortFlags = true

	var fs *pflag.FlagSet
	// 方法2：使用type switch
	switch typed := app.options.(type) {
	case NamedFlagSetOptions:
		var fss cliflag.NamedFlagSets
		fs = fss.FlagSet("global")

		if app.options != nil {
			fss = typed.Flags()
		}

		for _, f := range fss.FlagSets {
			cmd.Flags().AddFlagSet(f)
		}

		cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
		cliflag.SetUsageAndHelpFunc(cmd, fss, cols)
	case FlagSetOptions:
		fs = cmd.PersistentFlags()
		if app.options != nil {
			typed.AddFlags(fs)
		}
	default:
		// 即便没有 options，也要让 version 标志可用
		fs = cmd.PersistentFlags()
	}

	version.AddFlags(fs)

	if !app.noConfig {
		AddConfigFlag(fs, app.name, app.watch)
	}

	app.cmd = cmd
}

// Run 用于启动应用程序。
func (app *App) Run() {
	os.Exit(cli.Run(app.cmd))
}

func (app *App) runCommand(cmd *cobra.Command, args []string) error {
	// 显示应用程序版本信息
	version.PrintAndExitIfRequested()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	if app.options != nil {
		if err := viper.Unmarshal(app.options); err != nil {
			return err
		}

		if complete, ok := app.options.(interface{ Complete() error }); ok {
			if err := complete.Complete(); err != nil {
				return err
			}
		}

		if validate, ok := app.options.(interface{ Validate() error }); ok {
			if err := validate.Validate(); err != nil {
				return err
			}
		}
	}

	app.initializeLogger()

	if !app.silence {
		klog.InfoS("Starting application", "name", app.name, "version", version.Get().ToJSON())
		klog.InfoS("Golang settings", "GOGC", os.Getenv("GOGC"), "GOMAXPROCS", os.Getenv("GOMAXPROCS"), "GOTRACEBACK", os.Getenv("GOTRACEBACK"))
		if !app.noConfig {
			PrintConfig()
		} else if app.options != nil {
			cliflag.PrintFlags(cmd.Flags())
		}
	}

	if app.healthCheckFunc != nil {
		if err := app.healthCheckFunc(); err != nil {
			return err
		}
	}

	// 优先使用带 context 的 RunContextFunc，
	// 在收到 SIGINT/SIGTERM 时取消 context 让业务感知关停信号
	if app.runCtx != nil {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return app.runCtx(ctx)
	}

	return app.run()
}

// Command 返回应用程序内部的 cobra 命令实例。
func (app *App) Command() *cobra.Command {
	return app.cmd
}

// formatBaseName 根据给定的名称，格式化为不同操作系统下的可执行文件名。
func formatBaseName(name string) string {
	// 大小写不敏感，并在存在时去除可执行文件后缀
	if runtime.GOOS == "windows" {
		name = strings.ToLower(name)
		name = strings.TrimSuffix(name, ".exe")
	}
	return name
}

// initializeLogger 根据配置设置日志系统。
func (app *App) initializeLogger() {
	logOptions := log.NewOptions()

	// 从 viper 配置日志选项
	if viper.IsSet("log.disable-caller") {
		logOptions.DisableCaller = viper.GetBool("log.disable-caller")
	}
	if viper.IsSet("log.disable-stacktrace") {
		logOptions.DisableStacktrace = viper.GetBool("log.disable-stacktrace")
	}
	if viper.IsSet("log.level") {
		logOptions.Level = viper.GetString("log.level")
	}
	if viper.IsSet("log.format") {
		logOptions.Format = viper.GetString("log.format")
	}
	if viper.IsSet("log.output-paths") {
		logOptions.OutputPaths = viper.GetStringSlice("log.output-paths")
	}

	// 使用自定义上下文提取器初始化日志
	log.Init(logOptions, log.WithContextExtractor(app.contextExtractors))
}
