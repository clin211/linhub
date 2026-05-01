// Package log 提供基于 slog 标准库的统一日志实现。
//
// 设计目标：
//  1. 接口与历史 zap 实现保持兼容（Debugf/Infow/Errorw/W(ctx) 等签名不变），
//     便于业务代码无侵入升级。
//  2. 默认 handler 基于 slog 标准库（Text/JSON），可输出到 stdout/stderr/文件。
//  3. 可选启用 OpenTelemetry log bridge（otelslog），把日志同时送入 OTel
//     LoggerProvider，便于分布式可观测性聚合。
//  4. 与 GORM 深度集成：实现 gorm.io/gorm/logger.Interface。
package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	gormlogger "gorm.io/gorm/logger"
)

// ContextExtractors 把 context 中的字段提取为日志属性的映射。
// key 为日志中的字段名，value 为从 context 中读取该字段的函数。
type ContextExtractors map[string]func(context.Context) string

// Logger 定义 onex/linhub 项目的日志接口（兼容旧 zap 实现的方法签名）。
type Logger interface {
	Debugf(format string, args ...any)
	Debugw(msg string, keyvals ...any)
	Infof(format string, args ...any)
	Infow(msg string, keyvals ...any)
	Warnf(format string, args ...any)
	Warnw(msg string, keyvals ...any)
	Errorf(format string, args ...any)
	Errorw(err error, msg string, keyvals ...any)
	Panicf(format string, args ...any)
	Panicw(msg string, keyvals ...any)
	Fatalf(format string, args ...any)
	Fatalw(msg string, keyvals ...any)

	// W 解析传入的 context，提取关注的键值并加入到结构化日志中。
	W(ctx context.Context) Logger

	// AddCallerSkip 设置调用栈跳过层数（slog 通过 source 实现）。
	AddCallerSkip(skip int) Logger

	// Sync 刷新缓冲（slog 自身没有缓冲，此处保留为 no-op 以兼容 zap 行为）。
	Sync()

	// 集成 GORM logger.Interface
	gormlogger.Interface
}

// slogLogger 是 Logger 接口的 slog 实现。
type slogLogger struct {
	logger            *slog.Logger
	opts              *Options
	contextExtractors map[string]func(context.Context) string
}

// Option 用于配置 slogLogger。
type Option func(*slogLogger)

// 编译期断言：slogLogger 必须实现 Logger 接口。
var _ Logger = (*slogLogger)(nil)

var (
	mu  sync.Mutex
	std = newDefaultLogger()
)

// newDefaultLogger 返回基于默认 Options 的 logger，避免包初始化期 panic。
func newDefaultLogger() *slogLogger {
	logger, _ := newSlogLogger(NewOptions())
	if logger == nil {
		// 兜底：用一个简单的 stderr text handler
		logger = &slogLogger{
			logger:            slog.New(slog.NewTextHandler(os.Stderr, nil)),
			opts:              NewOptions(),
			contextExtractors: map[string]func(context.Context) string{},
		}
	}
	return logger
}

// WithContextExtractor 注册从 context 提取字段的逻辑。
func WithContextExtractor(extractors ContextExtractors) Option {
	return func(l *slogLogger) {
		for k, v := range extractors {
			l.contextExtractors[k] = v
		}
	}
}

// Init 使用指定的选项初始化全局 Logger。
// 与 zap 版本不同，本实现允许多次调用以热更新配置。
func Init(opts *Options, options ...Option) {
	mu.Lock()
	defer mu.Unlock()

	logger, err := newSlogLogger(opts, options...)
	if err != nil {
		// 初始化失败时打印到 stderr 并保留旧 logger，避免业务崩溃
		fmt.Fprintf(os.Stderr, "log.Init failed: %v; keeping previous logger\n", err)
		return
	}
	std = logger
	// 同步设置 slog 全局 default，便于第三方库（含 otelslog 自身）共用
	slog.SetDefault(logger.logger)
}

// NewLogger 根据传入的 opts 创建一个新的 Logger。
// 当 opts 为 nil 时使用默认配置。
func NewLogger(opts *Options, options ...Option) *slogLogger {
	logger, err := newSlogLogger(opts, options...)
	if err != nil || logger == nil {
		// 失败时返回一个使用默认 Options 的 logger
		fallback, _ := newSlogLogger(NewOptions(), options...)
		return fallback
	}
	return logger
}

// newSlogLogger 是带错误返回的内部构造函数。
func newSlogLogger(opts *Options, options ...Option) (*slogLogger, error) {
	if opts == nil {
		opts = NewOptions()
	}

	level, _ := parseLevel(opts.Level)

	writer, err := opts.resolveWriter()
	if err != nil {
		return nil, err
	}

	handlerOpts := &slog.HandlerOptions{
		AddSource: opts.AddSource,
		Level:     level,
	}

	var baseHandler slog.Handler
	switch Format(opts.Format) {
	case FormatJSON:
		baseHandler = slog.NewJSONHandler(writer, handlerOpts)
	default:
		baseHandler = slog.NewTextHandler(writer, handlerOpts)
	}

	// 启用 OTel bridge 时，包装 fanoutHandler：本地写一份 + OTel 发送一份
	if opts.EnableOTel {
		serviceName := opts.ServiceName
		if serviceName == "" {
			serviceName = "linhub"
		}
		otelHandler := otelslog.NewHandler(serviceName)
		baseHandler = fanoutHandler{handlers: []slog.Handler{baseHandler, otelHandler}}
	}

	logger := &slogLogger{
		logger:            slog.New(baseHandler),
		opts:              opts,
		contextExtractors: map[string]func(context.Context) string{},
	}

	for _, opt := range options {
		opt(logger)
	}

	return logger, nil
}

// Default 返回全局 Logger。
func Default() Logger { return std }

// Sync 是 slog 下的 no-op，保留以兼容旧 zap API。
func Sync() { std.Sync() }

func (l *slogLogger) Sync() { /* slog 无缓冲，无需 Sync */ }

// Options 返回当前 logger 使用的 Options（只读拷贝）。
func (l *slogLogger) Options() *Options {
	cp := *l.opts
	return &cp
}

// 包级便捷函数（委托给 std）。

func Debugf(format string, args ...any)            { std.Debugf(format, args...) }
func Debugw(msg string, keyvals ...any)            { std.Debugw(msg, keyvals...) }
func Infof(format string, args ...any)             { std.Infof(format, args...) }
func Infow(msg string, keyvals ...any)             { std.Infow(msg, keyvals...) }
func Warnf(format string, args ...any)             { std.Warnf(format, args...) }
func Warnw(msg string, keyvals ...any)             { std.Warnw(msg, keyvals...) }
func Errorf(format string, args ...any)            { std.Errorf(format, args...) }
func Errorw(err error, msg string, keyvals ...any) { std.Errorw(err, msg, keyvals...) }
func Panicf(format string, args ...any)            { std.Panicf(format, args...) }
func Panicw(msg string, keyvals ...any)            { std.Panicw(msg, keyvals...) }
func Fatalf(format string, args ...any)            { std.Fatalf(format, args...) }
func Fatalw(msg string, keyvals ...any)            { std.Fatalw(msg, keyvals...) }

// W 是包级便捷函数：基于 std 提取 context 字段并返回新 logger。
func W(ctx context.Context) Logger { return std.W(ctx) }

// AddCallerSkip 是包级便捷函数。
func AddCallerSkip(skip int) Logger { return std.AddCallerSkip(skip) }

// ===== sugar 实现（基于 slog） =====

func (l *slogLogger) logf(ctx context.Context, level slog.Level, format string, args ...any) {
	if !l.logger.Enabled(ctx, level) {
		return
	}
	l.logger.Log(ctx, level, fmt.Sprintf(format, args...))
}

func (l *slogLogger) logw(ctx context.Context, level slog.Level, msg string, keyvals ...any) {
	if !l.logger.Enabled(ctx, level) {
		return
	}
	l.logger.Log(ctx, level, msg, keyvals...)
}

func (l *slogLogger) Debugf(format string, args ...any) {
	l.logf(context.Background(), slog.LevelDebug, format, args...)
}

func (l *slogLogger) Debugw(msg string, keyvals ...any) {
	l.logw(context.Background(), slog.LevelDebug, msg, keyvals...)
}

func (l *slogLogger) Infof(format string, args ...any) {
	l.logf(context.Background(), slog.LevelInfo, format, args...)
}

func (l *slogLogger) Infow(msg string, keyvals ...any) {
	l.logw(context.Background(), slog.LevelInfo, msg, keyvals...)
}

func (l *slogLogger) Warnf(format string, args ...any) {
	l.logf(context.Background(), slog.LevelWarn, format, args...)
}

func (l *slogLogger) Warnw(msg string, keyvals ...any) {
	l.logw(context.Background(), slog.LevelWarn, msg, keyvals...)
}

func (l *slogLogger) Errorf(format string, args ...any) {
	l.logf(context.Background(), slog.LevelError, format, args...)
}

func (l *slogLogger) Errorw(err error, msg string, keyvals ...any) {
	if err != nil {
		keyvals = append(keyvals, "err", err.Error())
	}
	l.logw(context.Background(), slog.LevelError, msg, keyvals...)
}

// Panicf/Panicw 在记录后 panic（与 zap 对齐）。
func (l *slogLogger) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.logger.Log(context.Background(), slog.LevelError, msg)
	panic(msg)
}

func (l *slogLogger) Panicw(msg string, keyvals ...any) {
	l.logw(context.Background(), slog.LevelError, msg, keyvals...)
	panic(msg)
}

// Fatalf/Fatalw 在记录后 os.Exit(1)。
func (l *slogLogger) Fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.logger.Log(context.Background(), slog.LevelError, msg)
	os.Exit(1)
}

func (l *slogLogger) Fatalw(msg string, keyvals ...any) {
	l.logw(context.Background(), slog.LevelError, msg, keyvals...)
	os.Exit(1)
}

// W 把 context 中的字段提取为日志属性返回新 logger。
func (l *slogLogger) W(ctx context.Context) Logger {
	if ctx == nil || len(l.contextExtractors) == 0 {
		return l
	}

	cloned := *l
	attrs := make([]any, 0, len(l.contextExtractors)*2)
	for fieldName, extractor := range l.contextExtractors {
		if val := extractor(ctx); val != "" {
			attrs = append(attrs, fieldName, val)
		}
	}
	if len(attrs) > 0 {
		cloned.logger = l.logger.With(attrs...)
	}
	return &cloned
}

// AddCallerSkip 在 slog 模型中通过新建 logger 实现，
// 但由于 slog 默认通过 PC 记录调用位置，此实现保持 API 兼容即可。
func (l *slogLogger) AddCallerSkip(_ int) Logger {
	return l
}
