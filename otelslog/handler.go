package otelslog // import "go.opentelemetry.io/contrib/bridges/otelslog"

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// NewLogger 返回一个由新的 [Handler] 支撑的 [slog.Logger].
// 关于底层 Handler 的创建细节，请参见 [NewHandler].
func NewLogger(name string, options ...Option) *slog.Logger {
	return slog.New(NewHandler(name, options...))
}

type config struct {
	provider   log.LoggerProvider
	version    string
	schemaURL  string
	attributes []attribute.KeyValue
	source     bool
	level      slog.Level
}

func newConfig(options []Option) config {
	var c config
	for _, opt := range options {
		c = opt.apply(c)
	}

	if c.provider == nil {
		c.provider = global.GetLoggerProvider()
	}

	return c
}

func (c config) logger(name string) log.Logger {
	var opts []log.LoggerOption
	if c.version != "" {
		opts = append(opts, log.WithInstrumentationVersion(c.version))
	}
	if c.schemaURL != "" {
		opts = append(opts, log.WithSchemaURL(c.schemaURL))
	}
	if c.attributes != nil {
		opts = append(opts, log.WithInstrumentationAttributes(c.attributes...))
	}
	return c.provider.Logger(name, opts...)
}

// Option 用于配置 [Handler].
type Option interface {
	apply(config) config
}

type optFunc func(config) config

func (f optFunc) apply(c config) config { return f(c) }

// WithVersion 返回一个 [Option]，用于配置 [Handler] 所使用的
// [log.Logger] 的版本. 版本应当为正在记录日志的那个包的版本.
func WithVersion(version string) Option {
	return optFunc(func(c config) config {
		c.version = version
		return c
	})
}

// WithSchemaURL 返回一个 [Option]，用于配置 [Handler] 所使用的
// [log.Logger] 的语义约定 schema URL. schemaURL 应当为日志记录中
// 所使用的语义约定的 schema URL.
func WithSchemaURL(schemaURL string) Option {
	return optFunc(func(c config) config {
		c.schemaURL = schemaURL
		return c
	})
}

// WithAttributes 返回一个 [Option]，用于配置 [Handler] 所使用的
// [log.Logger] 的检测作用域（instrumentation scope）属性.
func WithAttributes(attributes ...attribute.KeyValue) Option {
	return optFunc(func(c config) config {
		c.attributes = attributes
		return c
	})
}

// WithLoggerProvider 返回一个 [Option]，用于配置 [Handler] 创建其
// [log.Logger] 时所使用的 [log.LoggerProvider].
//
// 默认情况下，如果未提供此 Option，则 Handler 会使用全局的
// LoggerProvider.
func WithLoggerProvider(provider log.LoggerProvider) Option {
	return optFunc(func(c config) config {
		c.provider = provider
		return c
	})
}

// WithSource 返回一个 [Option]，用于配置 [Handler] 在日志属性中
// 包含日志记录的源代码位置信息.
func WithSource(source bool) Option {
	return optFunc(func(c config) config {
		c.source = source
		return c
	})
}

// WithLevel 返回一个 [Option]，用于配置 Handler 的最低日志级别.
// 只有级别等于或高于该级别的日志记录才会被处理.
func WithLevel(level slog.Level) Option {
	return optFunc(func(c config) config {
		c.level = level
		return c
	})
}

// WithLevelString 返回一个 [Option]，使用字符串形式来配置 Handler
// 的最低日志级别. 支持的取值包括：
// "DEBUG"、"INFO"、"WARN"、"WARNING"、"ERROR".
// 如果传入不支持的级别，则默认使用 INFO 级别.
func WithLevelString(levelStr string) Option {
	level := parseLevelString(levelStr)
	return WithLevel(level)
}

// parseLevelString 将字符串转换为 slog.Level.
func parseLevelString(levelStr string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(levelStr)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		// 未知级别时默认为 INFO.
		return slog.LevelInfo
	}
}

// Handler 是一个 [slog.Handler]，它会将所接收的所有日志记录发送给
// OpenTelemetry. 关于转换方式，请参见包文档.
type Handler struct {
	// 通过显式将其设为不可比较，以确保向前兼容性.
	noCmp [0]func() //nolint:unused  // 此字段确实被使用了.

	attrs  *kvBuffer
	group  *group
	logger log.Logger
	level  slog.Level // 添加最低日志级别字段

	source bool
}

// 编译期检查：*Handler 实现了 slog.Handler 接口.
var _ slog.Handler = (*Handler)(nil)

// NewHandler 返回一个新的 [Handler]，可作为 [slog.Handler] 使用.
//
// 如果未提供 [WithLoggerProvider]，返回的 Handler 将使用全局的
// LoggerProvider.
//
// 提供的 name 需要唯一标识正在被记录日志的代码. 通常情况下，
// 这就是代码的包名. 如果 name 为空，[log.Logger] 的实现可能会
// 用默认值覆盖该值.
func NewHandler(name string, options ...Option) *Handler {
	cfg := newConfig(options)
	return &Handler{
		logger: cfg.logger(name),
		source: cfg.source,
		level:  cfg.level, // 设置最低日志级别
	}
}

// Handle 处理传入的日志记录.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	h.logger.Emit(ctx, h.convertRecord(record))
	return nil
}

func (h *Handler) convertRecord(r slog.Record) log.Record {
	var record log.Record
	record.SetTimestamp(r.Time)
	record.SetBody(log.StringValue(r.Message))

	const sevOffset = slog.Level(log.SeverityDebug) - slog.LevelDebug
	record.SetSeverity(log.Severity(r.Level + sevOffset))
	record.SetSeverityText(r.Level.String())

	if h.source {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		record.AddAttributes(
			log.String(string(semconv.CodeFilePathKey), f.File),
			log.String(string(semconv.CodeFunctionNameKey), f.Function),
			log.Int(string(semconv.CodeLineNumberKey), f.Line),
		)
	}

	if h.attrs.Len() > 0 {
		record.AddAttributes(h.attrs.KeyValues()...)
	}

	n := r.NumAttrs()
	if h.group != nil {
		if n > 0 {
			buf := newKVBuffer(n)
			r.Attrs(buf.AddAttr)
			record.AddAttributes(h.group.KeyValue(buf.KeyValues()...))
		} else {
			// 如果没有任何属性，Handler 不应输出分组.
			g := h.group.NextNonEmpty()
			if g != nil {
				record.AddAttributes(g.KeyValue())
			}
		}
	} else if n > 0 {
		buf := newKVBuffer(n)
		r.Attrs(buf.AddAttr)
		record.AddAttributes(buf.KeyValues()...)
	}

	return record
}

// Enabled 在 Handler 针对所提供的上下文和级别启用日志记录时返回 true.
// 否则，如果未启用则返回 false.
//
// 决策顺序：
//  1. 先让底层 OTel log.Logger 决定（它能根据 ctx 中的信号、provider 配置等做精细化判断）；
//     例如调用方可以在 ctx 中放置 sampling/debug 标记，使原本被本地级别过滤的日志被放行。
//  2. 底层 logger 拒绝时，再看本地配置的最小级别 h.level 是否允许。
//
// 这样可以避免"未配置 WithLevel 时默认 h.level=LevelInfo 而无意中丢弃所有 Debug 日志"
// 的陷阱（当底层 logger 实际允许时）。
func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool {
	const sevOffset = slog.Level(log.SeverityDebug) - slog.LevelDebug
	param := log.EnabledParameters{Severity: log.Severity(l + sevOffset)}

	if h.logger.Enabled(ctx, param) {
		return true
	}
	return l >= h.level
}

// WithAttrs 基于 h 返回一个新的 [slog.Handler]，该 Handler 在记录日志时
// 会使用传入的 attrs.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	if h2.group != nil {
		h2.group = h2.group.Clone()
		h2.group.AddAttrs(attrs)
	} else {
		if h2.attrs == nil {
			h2.attrs = newKVBuffer(len(attrs))
		} else {
			h2.attrs = h2.attrs.Clone()
		}
		h2.attrs.AddAttrs(attrs)
	}
	return &h2
}

// WithGroup 基于 h 返回一个新的 [slog.Handler]，该 Handler 会将所有消息
// 和属性记录在以指定名称命名的分组中.
func (h *Handler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.group = &group{name: name, next: h2.group}
	return &h2
}

// group 表示一个从 slog 接收到的分组.
type group struct {
	// name 是分组的名称.
	name string
	// attrs 是与该分组关联的属性.
	attrs *kvBuffer
	// next 指向包含该分组的下一个分组.
	//
	// 分组在 OpenTelemetry 中以 map 值类型表示. 这意味着，
	// 对于如下的 slog 分组层级 ...
	//
	//   WithGroup("G").WithGroup("H").WithGroup("I")
	//
	// 对应的 OpenTelemetry 日志值类型将具有如下的层级结构 ...
	//
	//   KeyValue{
	//     Key: "G",
	//     Value: []KeyValue{{
	//       Key: "H",
	//       Value: []KeyValue{{
	//         Key: "I",
	//         Value: []KeyValue{},
	//       }},
	//     }},
	//   }
	//
	// 当属性被记录时（例如 Info("msg", "key", "value") 或
	// WithAttrs("key", "value")），需要将它们添加到"叶子"分组中.
	// 在上述示例中，叶子分组就是 "I"：
	//
	//   KeyValue{
	//     Key: "G",
	//     Value: []KeyValue{{
	//       Key: "H",
	//       Value: []KeyValue{{
	//         Key: "I",
	//         Value: []KeyValue{
	//           String("key", "value"),
	//         },
	//       }},
	//     }},
	//   }
	//
	// 因此，分组以链表结构组织，其中"叶子"节点位于链表头部.
	// 沿用上述示例，分组的数据表示形式为 ...
	//
	//   *group{"I", next: *group{"H", next: *group{"G"}}}
	next *group
}

// NextNonEmpty 返回 g 所在链表中下一个带有属性的分组（包括 g 本身）.
// 如果找不到这样的分组，则返回 nil.
func (g *group) NextNonEmpty() *group {
	if g == nil || g.attrs.Len() > 0 {
		return g
	}
	return g.next.NextNonEmpty()
}

// KeyValue 以 [log.KeyValue] 的形式返回包含 kvs 的分组 g.
// 返回的 KeyValue 的值类型为 [log.KindMap].
//
// 传入的 kvs 会被渲染到返回值中，但不会被添加到该分组.
//
// 该方法不会检查 g. 调用方有责任确保 g 非空或者 kvs 非空，
// 以便返回一个有效的分组表示（按照 slog 的要求）.
func (g *group) KeyValue(kvs ...log.KeyValue) log.KeyValue {
	// 假设对分组 g 的检查已经完成（即非空）.
	out := log.Map(g.name, g.attrs.KeyValues(kvs...)...)
	g = g.next
	for g != nil {
		// 如果没有任何属性，Handler 不应输出分组.
		if g.attrs.Len() > 0 {
			out = log.Map(g.name, g.attrs.KeyValues(out)...)
		}
		g = g.next
	}
	return out
}

// Clone 返回 g 的一份拷贝.
func (g *group) Clone() *group {
	if g == nil {
		return nil
	}
	g2 := *g
	g2.attrs = g2.attrs.Clone()
	return &g2
}

// AddAttrs 将 attrs 添加到 g 中.
func (g *group) AddAttrs(attrs []slog.Attr) {
	if g.attrs == nil {
		g.attrs = newKVBuffer(len(attrs))
	}
	g.attrs.AddAttrs(attrs)
}

type kvBuffer struct {
	data []log.KeyValue
}

func newKVBuffer(n int) *kvBuffer {
	return &kvBuffer{data: make([]log.KeyValue, 0, n)}
}

// Len 返回 b 所持有的 [log.KeyValue] 数量.
func (b *kvBuffer) Len() int {
	if b == nil {
		return 0
	}
	return len(b.data)
}

// Clone 返回 b 的一份拷贝.
func (b *kvBuffer) Clone() *kvBuffer {
	if b == nil {
		return nil
	}
	return &kvBuffer{data: slices.Clone(b.data)}
}

// KeyValues 返回将 kvs 追加到 b 持有的 [log.KeyValue] 之后的结果.
func (b *kvBuffer) KeyValues(kvs ...log.KeyValue) []log.KeyValue {
	if b == nil {
		return kvs
	}
	return append(b.data, kvs...)
}

// AddAttrs 将 attrs 添加到 b 中.
func (b *kvBuffer) AddAttrs(attrs []slog.Attr) {
	b.data = slices.Grow(b.data, len(attrs))
	for _, a := range attrs {
		_ = b.AddAttr(a)
	}
}

// AddAttr 将 attr 添加到 b 中并返回 true.
//
// 该方法被设计为可以传给 [slog.Record] 的 AddAttributes 方法使用.
//
// 如果 attr 是一个 key 为空的分组，则其值会被展平.
//
// 如果 attr 为空，则会被丢弃.
func (b *kvBuffer) AddAttr(attr slog.Attr) bool {
	if attr.Key == "" {
		if attr.Value.Kind() == slog.KindGroup {
			// Handler 应当内联（展平）一个 key 为空的分组的属性.
			for _, a := range attr.Value.Group() {
				b.data = append(b.data, log.KeyValue{
					Key:   a.Key,
					Value: convert(a.Value),
				})
			}
			return true
		}

		if attr.Value.Any() == nil {
			// Handler 应当忽略一个空的 Attr.
			return true
		}
	}
	b.data = append(b.data, log.KeyValue{
		Key:   attr.Key,
		Value: convert(attr.Value),
	})
	return true
}

func convert(v slog.Value) log.Value {
	switch v.Kind() {
	case slog.KindAny:
		return convertValue(v.Any())
	case slog.KindBool:
		return log.BoolValue(v.Bool())
	case slog.KindDuration:
		return log.Int64Value(v.Duration().Nanoseconds())
	case slog.KindFloat64:
		return log.Float64Value(v.Float64())
	case slog.KindInt64:
		return log.Int64Value(v.Int64())
	case slog.KindString:
		return log.StringValue(v.String())
	case slog.KindTime:
		return log.Int64Value(v.Time().UnixNano())
	case slog.KindUint64:
		const maxInt64 = ^uint64(0) >> 1
		u := v.Uint64()
		if u > maxInt64 {
			return log.Float64Value(float64(u))
		}
		return log.Int64Value(int64(u))
	case slog.KindGroup:
		g := v.Group()
		buf := newKVBuffer(len(g))
		buf.AddAttrs(g)
		return log.MapValue(buf.data...)
	case slog.KindLogValuer:
		return convert(v.Resolve())
	default:
		// 尝试尽可能优雅地处理这种情况.
		//
		// 此处不要 panic. 我们的目标是：当有新的 slog.Kind 被添加时，
		// 让开发者首先发现这一点. 针对新增 Kind 的测试会发现这种
		// 格式异常的属性，效果与 panic 类似. 不过，让用户提 issue
		// 询问为什么他们的属性带有 "unhandled: " 前缀，比让他们的
		// 代码直接 panic 要好得多.
		return log.StringValue(fmt.Sprintf("unhandled: (%s) %+v", v.Kind(), v.Any()))
	}
}
