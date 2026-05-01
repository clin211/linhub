package where

import (
	"context"
	"fmt"
	"sync/atomic"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// defaultLimit 定义分页的默认限制。-1 在 GORM 中表示"清除限制"。
	defaultLimit = -1
)

// =========================================================================
// 类型定义
//
// 命名规则：所有"概念性数据结构"加上语义后缀（Spec/Cond/Clause），
// 让方法层与包级函数能够使用 SQL 关键字作为名称（Where/Query/Tenant/Join/Preload）。
// =========================================================================

// TenantSpec 描述一个已注册的租户：键 + 上下文取值函数。
type TenantSpec struct {
	Key       string                           // 与租户关联的键（列名）
	ValueFunc func(ctx context.Context) string // 根据上下文获取租户值的函数
}

// Where 定义一个能修改 GORM 查询的应用器。
//
// 接口的 Where 方法将自身携带的所有条件应用到给定的 *gorm.DB 实例并返回新实例。
// 命名保留为 Where 以与历史 API 完全兼容；"添加 WHERE 条件"的链式方法以 Filter 区分。
type Where interface {
	Where(db *gorm.DB) *gorm.DB
}

// QueryCond 表示一条带参数的原始 WHERE 查询条件，对应 GORM `db.Where(query, args...)`。
type QueryCond struct {
	// Query 持有要在 GORM 查询中使用的条件。
	// 可以是字符串、map、结构体或其他 GORM Where 子句支持的类型。
	Query interface{}

	// Args 持有将传递给查询条件的参数。
	// 这些值将用于替换查询中的占位符。
	Args []interface{}
}

// JoinClause 表示一条 JOIN 子句，对应 GORM `Joins(query, args...)`。
//
// 使用示例：
//
//	whr.Join("LEFT JOIN posts ON posts.user_id = users.id")
//	whr.Join("LEFT JOIN posts ON posts.user_id = users.id AND posts.published = ?", true)
type JoinClause struct {
	Query string
	Args  []interface{}
}

// PreloadSpec 表示一个要预加载的关联，对应 GORM `Preload(association, args...)`。
//
// args 可以是带占位符的字符串 + 参数，或 `func(db *gorm.DB) *gorm.DB` 形式的过滤回调。
//
// 使用示例：
//
//	whr.Preload("Posts")                                   // 简单预加载
//	whr.Preload("Posts", "published = ?", true)            // 带条件
//	whr.Preload("Posts", func(db *gorm.DB) *gorm.DB {      // 函数式条件
//	    return db.Order("id desc").Limit(5)
//	})
type PreloadSpec struct {
	Association string
	Args        []interface{}
}

// Option 定义一个修改 Options 的函数类型。
type Option func(*Options)

// Options 持有 GORM 查询条件的选项。
//
// 设计说明：
//   - 该结构体被设计为可重复应用：同一个 Options 实例可安全地传给多次 Apply 调用，
//     不会产生条件累积。
//   - Selects/Omits/Orders 等字段仅在显式设置时下推到 GORM，避免对 update/delete
//     链路造成无意义的字段约束。
type Options struct {
	// Offset 定义分页的起始点。
	// +optional
	Offset int `json:"offset"`
	// Limit 定义返回结果的最大数量。-1 表示不限制。
	// +optional
	Limit int `json:"limit"`
	// Filters 包含用于过滤记录的键值对（AND 关系）。
	// 推荐使用 string 作为 key（列名）。
	Filters map[any]any
	// Clauses 包含要附加到查询中的自定义子句。
	Clauses []clause.Expression
	// Queries 包含要执行的查询条件列表（query, args...）。
	Queries []QueryCond
	// Selects 仅 SELECT/UPDATE 这些字段（对应 GORM `Select`）。
	// 在更新链路中配合 store.UpdateColumns 可实现"只更新某几个字段"。
	Selects []string
	// Omits 排除这些字段（对应 GORM `Omit`）。
	Omits []string
	// Orders 排序表达式列表，例如 "created_at desc"。
	Orders []string
	// Joins 是 JOIN 子句列表（对应 GORM `Joins`）。
	Joins []JoinClause
	// Preloads 是预加载关联列表（对应 GORM `Preload`）。
	// 注意：Preloads 不影响主表 Count，CountOptions 会将其剥离。
	Preloads []PreloadSpec
	// Groups 是 GROUP BY 列名列表（对应 GORM `Group`）。
	Groups []string
	// Havings 是 HAVING 条件列表（对应 GORM `Having`）。
	Havings []QueryCond
}

// registeredTenant 持有已注册的租户实例（线程安全）。
//
// 设计说明：
//   - 使用 atomic.Pointer 保证 RegisterTenant 与 Tenant() 之间的并发安全；
//   - 读路径无锁，性能与直接读 var 等价；
//   - 始终存储非 nil 指针（即便用户传入空 key + nil func，也存一个零值 TenantSpec），
//     以简化读路径的判空逻辑。
var registeredTenant atomic.Pointer[TenantSpec]

func init() {
	registeredTenant.Store(&TenantSpec{})
}

// =========================================================================
// Option 函数（用于 NewWhere(opts...) 构造）
// =========================================================================

// WithOffset 使用给定的 offset 值初始化 Options 中的 Offset 字段。
func WithOffset(offset int64) Option {
	return func(whr *Options) {
		if offset < 0 {
			offset = 0
		}
		whr.Offset = int(offset)
	}
}

// WithLimit 使用给定的 limit 值初始化 Options 中的 Limit 字段。
func WithLimit(limit int64) Option {
	return func(whr *Options) {
		if limit <= 0 {
			limit = defaultLimit
		}
		whr.Limit = int(limit)
	}
}

// WithPage 是一个语法糖函数，用于将 page 和 pageSize 转换为 Options 中的 limit 和 offset。
func WithPage(page int, pageSize int) Option {
	return func(whr *Options) {
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = defaultLimit
		}
		whr.Offset = (page - 1) * pageSize
		whr.Limit = pageSize
	}
}

// WithFilter 使用给定的过滤条件 map 初始化 Options 中的 Filters 字段。
func WithFilter(filter map[any]any) Option {
	return func(whr *Options) {
		whr.Filters = filter
	}
}

// WithClauses 将子句附加到 Options 中的 Clauses 字段。
func WithClauses(conds ...clause.Expression) Option {
	return func(whr *Options) {
		whr.Clauses = append(whr.Clauses, conds...)
	}
}

// WithQuery 创建一个 Option，用于将带参数的查询条件添加到 Options 结构体中。
// query 参数可以是字符串、map、结构体或 GORM Where 子句支持的任何其他类型。
// args 参数包含将替换查询字符串中占位符的值。
func WithQuery(query interface{}, args ...interface{}) Option {
	return func(whr *Options) {
		whr.Queries = append(whr.Queries, QueryCond{Query: query, Args: args})
	}
}

// WithSelect 设置 SELECT/UPDATE 时的目标列。
// 配合 store.UpdateColumns 可实现"只更新某几个字段"。
func WithSelect(columns ...string) Option {
	return func(whr *Options) {
		whr.Selects = append(whr.Selects, columns...)
	}
}

// WithOmit 设置要排除的列。
func WithOmit(columns ...string) Option {
	return func(whr *Options) {
		whr.Omits = append(whr.Omits, columns...)
	}
}

// WithOrder 添加 ORDER BY 表达式，例如 "created_at desc"。多次调用按顺序累积。
func WithOrder(orders ...string) Option {
	return func(whr *Options) {
		whr.Orders = append(whr.Orders, orders...)
	}
}

// WithJoin 添加一条 JOIN 子句。可多次调用，顺序累积。
func WithJoin(query string, args ...interface{}) Option {
	return func(whr *Options) {
		whr.Joins = append(whr.Joins, JoinClause{Query: query, Args: args})
	}
}

// WithPreload 添加一个预加载关联及其条件。可多次调用，顺序累积。
func WithPreload(association string, args ...interface{}) Option {
	return func(whr *Options) {
		whr.Preloads = append(whr.Preloads, PreloadSpec{Association: association, Args: args})
	}
}

// WithGroup 添加 GROUP BY 列。可多次调用，顺序累积。
func WithGroup(columns ...string) Option {
	return func(whr *Options) {
		whr.Groups = append(whr.Groups, columns...)
	}
}

// WithHaving 添加一条 HAVING 条件。可多次调用，顺序累积。
func WithHaving(query interface{}, args ...interface{}) Option {
	return func(whr *Options) {
		whr.Havings = append(whr.Havings, QueryCond{Query: query, Args: args})
	}
}

// NewWhere 构造一个新的 Options 对象，并应用给定的 where 选项。
func NewWhere(opts ...Option) *Options {
	whr := &Options{
		Offset:   0,
		Limit:    defaultLimit,
		Filters:  map[any]any{},
		Clauses:  make([]clause.Expression, 0),
		Queries:  make([]QueryCond, 0),
		Selects:  make([]string, 0),
		Omits:    make([]string, 0),
		Orders:   make([]string, 0),
		Joins:    make([]JoinClause, 0),
		Preloads: make([]PreloadSpec, 0),
		Groups:   make([]string, 0),
		Havings:  make([]QueryCond, 0),
	}

	for _, opt := range opts {
		opt(whr)
	}

	return whr
}

// =========================================================================
// 链式方法
//
// 命名规则：方法名尽可能与 SQL 关键字对齐：
//   Where / Select / Omit / Order / Join / Group / Having / Preload。
// 因 Options.Offset / Options.Limit 字段同名，方法层只能保留简写 O / L。
// 历史简写（P/F/C/Q/T）保留为兼容别名，已在业务代码中广泛使用。
// =========================================================================

// O 设置查询偏移量。由于 Options.Offset 字段同名，方法层只能使用简写 O。
func (whr *Options) O(offset int) *Options {
	if offset < 0 {
		offset = 0
	}
	whr.Offset = offset
	return whr
}

// L 设置查询限制数量。由于 Options.Limit 字段同名，方法层只能使用简写 L。
func (whr *Options) L(limit int) *Options {
	if limit <= 0 {
		limit = defaultLimit
	}
	whr.Limit = limit
	return whr
}

// Page 根据页码和每页大小设置分页（offset = (page-1)*pageSize, limit = pageSize）。
func (whr *Options) Page(page int, pageSize int) *Options {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultLimit
	}
	whr.Offset = (page - 1) * pageSize
	whr.Limit = pageSize
	return whr
}

// P 是 Page 的历史简写别名（为兼容存量代码保留，新代码请使用 Page）。
func (whr *Options) P(page int, pageSize int) *Options { return whr.Page(page, pageSize) }

// Filter 添加键值形式的 WHERE 条件（AND 关系，键值成对出现）。
//
// 命名保留为 Filter 而非 Where，避免与"应用所有条件到 db"的 Where(db) 方法冲突。
//
// 契约：
//   - kvs 必须成对出现：key, value, key, value, ...
//   - key 必须是 string（列名）；其他类型在 map[any]any 中会因
//     interface 比较语义产生难以察觉的键碰撞（如 int(1) ≠ int64(1)），
//     视为编程错误并 panic，以阻止生产事故。
//
// 错误处理：
//   - 奇数个参数：panic（编程错误，必须在测试期暴露）；
//   - 非 string key：panic（同上）；
//   - 这两类错误属于"使用方式错误"，相对于静默丢弃条件、
//     生成"无 WHERE 子句的全表扫描 SQL"导致的安全/性能事故，
//     panic 是更安全的设计选择。
func (whr *Options) Filter(kvs ...any) *Options {
	if len(kvs)%2 != 0 {
		panic(fmt.Sprintf("where.Filter: odd number of args (%d), must be key-value pairs", len(kvs)))
	}

	if whr.Filters == nil {
		whr.Filters = map[any]any{}
	}

	for i := 0; i < len(kvs); i += 2 {
		key, ok := kvs[i].(string)
		if !ok {
			panic(fmt.Sprintf("where.Filter: key at index %d must be string column name, got %T", i, kvs[i]))
		}
		whr.Filters[key] = kvs[i+1]
	}

	return whr
}

// F 是 Filter 的历史简写别名（为兼容存量代码保留，新代码请使用 Filter）。
func (whr *Options) F(kvs ...any) *Options { return whr.Filter(kvs...) }

// Clause 向查询添加 GORM 子句。
func (whr *Options) Clause(conds ...clause.Expression) *Options {
	whr.Clauses = append(whr.Clauses, conds...)
	return whr
}

// C 是 Clause 的历史简写别名（为兼容存量代码保留，新代码请使用 Clause）。
func (whr *Options) C(conds ...clause.Expression) *Options { return whr.Clause(conds...) }

// Query 添加一条带参数的原始查询条件，等价于 GORM `db.Where(query, args...)`。
//
// 注意：方法名 Query 与同包类型 `QueryCond` 不冲突；方法位于 receiver 命名空间。
func (whr *Options) Query(query interface{}, args ...interface{}) *Options {
	whr.Queries = append(whr.Queries, QueryCond{Query: query, Args: args})
	return whr
}

// Q 是 Query 的历史简写别名（为兼容存量代码保留，新代码请使用 Query）。
func (whr *Options) Q(query interface{}, args ...interface{}) *Options {
	return whr.Query(query, args...)
}

// Tenant 应用已注册的租户键值到 Filters。
//
// 如果未通过 RegisterTenant 注册租户，本方法不做任何事。
func (whr *Options) Tenant(ctx context.Context) *Options {
	t := registeredTenant.Load()
	if t != nil && t.Key != "" && t.ValueFunc != nil {
		whr.Filter(t.Key, t.ValueFunc(ctx))
	}
	return whr
}

// T 是 Tenant 的历史简写别名（为兼容存量代码保留，新代码请使用 Tenant）。
func (whr *Options) T(ctx context.Context) *Options { return whr.Tenant(ctx) }

// Select 设置 SELECT/UPDATE 时的目标列。
func (whr *Options) Select(columns ...string) *Options {
	whr.Selects = append(whr.Selects, columns...)
	return whr
}

// Omit 设置要排除的列。
func (whr *Options) Omit(columns ...string) *Options {
	whr.Omits = append(whr.Omits, columns...)
	return whr
}

// Order 添加 ORDER BY 表达式，例如 "created_at desc"。
func (whr *Options) Order(orders ...string) *Options {
	whr.Orders = append(whr.Orders, orders...)
	return whr
}

// Join 添加 JOIN 子句。
func (whr *Options) Join(query string, args ...interface{}) *Options {
	whr.Joins = append(whr.Joins, JoinClause{Query: query, Args: args})
	return whr
}

// Preload 添加预加载关联。
func (whr *Options) Preload(association string, args ...interface{}) *Options {
	whr.Preloads = append(whr.Preloads, PreloadSpec{Association: association, Args: args})
	return whr
}

// Group 添加 GROUP BY 列。
func (whr *Options) Group(columns ...string) *Options {
	whr.Groups = append(whr.Groups, columns...)
	return whr
}

// Having 添加 HAVING 条件。
func (whr *Options) Having(query interface{}, args ...interface{}) *Options {
	whr.Havings = append(whr.Havings, QueryCond{Query: query, Args: args})
	return whr
}

// CountOptions 返回适合 Count 查询的浅拷贝。
//
// 保留：过滤、子句、原始查询、Joins、Groups、Havings —— 这些会影响 COUNT 结果。
// 丢弃：Offset/Limit、Orders、Selects、Omits、Preloads —— 这些仅影响结果展示，不影响 Count。
//
// 注意：当存在 Groups 时，GORM 的 Count 行为与无 Group 时不同（会 Count 分组数）。
// 该方法只负责忠实下推条件，分组下的 Count 语义需调用方自行确认。
func (whr *Options) CountOptions() *Options {
	return &Options{
		Offset:  0,
		Limit:   defaultLimit,
		Filters: whr.Filters,
		Clauses: whr.Clauses,
		Queries: whr.Queries,
		Joins:   whr.Joins,
		Groups:  whr.Groups,
		Havings: whr.Havings,
	}
}

// Where 将所有条件应用到给定的 *gorm.DB 实例并返回新实例（实现 Where 接口）。
//
// 实现要点：
//   - 该方法是幂等的——多次调用同一 Options 不会产生条件叠加。
//   - 各字段仅在显式设置时下推；Limit 在显式正值或 -1 时下推（-1 表示"清除限制"）。
//   - 对于 update/delete 链路，建议调用方仅传入过滤条件，不要预先设置
//     Offset/Limit/Order/Group 等，避免 GORM 警告或意外行为。
//
// 应用顺序遵循 SQL 语义：JOIN → WHERE → GROUP BY → HAVING → SELECT/OMIT → ORDER → LIMIT/OFFSET。
// Preload 是 GORM 的"额外查询"，不影响主 SQL 但会在结果填充时执行。
func (whr *Options) Where(db *gorm.DB) *gorm.DB {
	for _, j := range whr.Joins {
		db = db.Joins(j.Query, j.Args...)
	}
	if len(whr.Filters) > 0 {
		db = db.Where(whr.Filters)
	}
	for _, q := range whr.Queries {
		db = db.Where(q.Query, q.Args...)
	}
	if len(whr.Clauses) > 0 {
		db = db.Clauses(whr.Clauses...)
	}
	for _, g := range whr.Groups {
		db = db.Group(g)
	}
	for _, h := range whr.Havings {
		db = db.Having(h.Query, h.Args...)
	}
	for _, p := range whr.Preloads {
		db = db.Preload(p.Association, p.Args...)
	}
	if len(whr.Selects) > 0 {
		db = db.Select(whr.Selects)
	}
	if len(whr.Omits) > 0 {
		db = db.Omit(whr.Omits...)
	}
	for _, o := range whr.Orders {
		db = db.Order(o)
	}
	if whr.Offset > 0 {
		db = db.Offset(whr.Offset)
	}
	if whr.Limit > 0 || whr.Limit == defaultLimit {
		db = db.Limit(whr.Limit)
	}
	return db
}

// =========================================================================
// 包级便捷构造函数
//
// 命名规则：所有便捷构造函数提供与 SQL 关键字对齐的完整命名；
// 历史简写（O/L/P/F/C/Q/T）保留为兼容别名。
// =========================================================================

// Offset 创建一个仅设置偏移量的新 Options。
func Offset(offset int) *Options { return NewWhere().O(offset) }

// O 是 Offset 的历史简写别名（为兼容存量代码保留，新代码请使用 Offset）。
func O(offset int) *Options { return Offset(offset) }

// Limit 创建一个仅设置限制数量的新 Options。
func Limit(limit int) *Options { return NewWhere().L(limit) }

// L 是 Limit 的历史简写别名（为兼容存量代码保留，新代码请使用 Limit）。
func L(limit int) *Options { return Limit(limit) }

// Page 创建一个仅设置分页的新 Options。
func Page(page int, pageSize int) *Options { return NewWhere().Page(page, pageSize) }

// P 是 Page 的历史简写别名（为兼容存量代码保留，新代码请使用 Page）。
func P(page int, pageSize int) *Options { return Page(page, pageSize) }

// Filter 创建一个带 WHERE 条件的新 Options（kvs 为成对的 key,value）。
//
// 包级名称保留为 Filter（避免与同包接口 Where 冲突）。
func Filter(kvs ...any) *Options { return NewWhere().Filter(kvs...) }

// F 是 Filter 的历史简写别名（为兼容存量代码保留，新代码请使用 Filter）。
func F(kvs ...any) *Options { return Filter(kvs...) }

// Clause 创建一个带 GORM 子句的新 Options。
func Clause(conds ...clause.Expression) *Options { return NewWhere().Clause(conds...) }

// C 是 Clause 的历史简写别名（为兼容存量代码保留，新代码请使用 Clause）。
func C(conds ...clause.Expression) *Options { return Clause(conds...) }

// Query 创建一个带原始查询条件的新 Options。
func Query(query interface{}, args ...interface{}) *Options {
	return NewWhere().Query(query, args...)
}

// Q 是 Query 的历史简写别名（为兼容存量代码保留，新代码请使用 Query）。
func Q(query interface{}, args ...interface{}) *Options { return Query(query, args...) }

// Tenant 创建一个应用已注册租户的新 Options。
func Tenant(ctx context.Context) *Options { return NewWhere().Tenant(ctx) }

// T 是 Tenant 的历史简写别名（为兼容存量代码保留，新代码请使用 Tenant）。
func T(ctx context.Context) *Options { return Tenant(ctx) }

// Select 创建一个仅设置 Select 列的新 Options。
func Select(columns ...string) *Options { return NewWhere().Select(columns...) }

// Omit 创建一个仅设置 Omit 列的新 Options。
func Omit(columns ...string) *Options { return NewWhere().Omit(columns...) }

// Order 创建一个仅设置 Order 表达式的新 Options。
func Order(orders ...string) *Options { return NewWhere().Order(orders...) }

// Join 创建一个仅设置 JOIN 子句的新 Options。
func Join(query string, args ...interface{}) *Options { return NewWhere().Join(query, args...) }

// Preload 创建一个仅设置 Preload 的新 Options。
func Preload(association string, args ...interface{}) *Options {
	return NewWhere().Preload(association, args...)
}

// Group 创建一个仅设置 GROUP BY 列的新 Options。
func Group(columns ...string) *Options { return NewWhere().Group(columns...) }

// Having 创建一个仅设置 HAVING 条件的新 Options。
func Having(query interface{}, args ...interface{}) *Options {
	return NewWhere().Having(query, args...)
}

// =========================================================================
// 租户注册
// =========================================================================

// RegisterTenant 注册一个新租户，使用指定的键和值函数。
//
// 并发安全：本函数与 (*Options).Tenant / 包级 Tenant 之间通过 atomic.Pointer 同步，
// 可在任意 goroutine 中调用。传入空 key + nil func 等价于"清空注册"。
func RegisterTenant(key string, valueFunc func(context.Context) string) {
	registeredTenant.Store(&TenantSpec{
		Key:       key,
		ValueFunc: valueFunc,
	})
}
