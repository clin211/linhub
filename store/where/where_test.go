package where_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/clin211/linhub/store/where"
)

// testModel 是仅用于测试的轻量模型，覆盖足够多的列以验证 Select/Omit。
type testModel struct {
	ID        int    `gorm:"primaryKey"`
	Name      string `gorm:"size:64"`
	Age       int
	Password  string `gorm:"size:64"`
	CreatedAt int64
}

// newDB 启动一个内存 sqlite 库（纯 Go，无需 CGO），用于 ToSQL 验证。
func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&testModel{}))
	return db
}

// toSQL 是 db.ToSQL 的快捷封装，便于断言。
func toSQL(db *gorm.DB, fn func(tx *gorm.DB) *gorm.DB) string {
	return db.ToSQL(fn)
}


// ==============================================================
// 一、Bug 修复验证：Where() 必须幂等
// ==============================================================

// TestWhere_Idempotent 验证修复后的 Where() 不再有副作用：
// 同一个 Options 重复调用 Where()，生成的 SQL 完全一致，
// 且 whr.Clauses 不会被附加 Queries 转换出来的条件。
func TestWhere_Idempotent(t *testing.T) {
	db := newDB(t)

	whr := where.NewWhere().
		F("name", "alice").
		Q("age > ?", 18).
		C(clause.OrderBy{Columns: []clause.OrderByColumn{{Column: clause.Column{Name: "id"}, Desc: true}}})

	clausesBefore := len(whr.Clauses)
	sql1 := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	clausesAfterFirst := len(whr.Clauses)

	sql2 := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	clausesAfterSecond := len(whr.Clauses)

	assert.Equal(t, sql1, sql2, "Where() 必须是幂等的，多次调用产出相同 SQL")
	assert.Equal(t, clausesBefore, clausesAfterFirst, "Where() 不应修改 whr.Clauses")
	assert.Equal(t, clausesBefore, clausesAfterSecond, "Where() 多次调用后 whr.Clauses 仍不应改变")
}

// TestWhere_QueriesNotAccumulated 显式验证旧 Bug 场景：
// 当 Options 同时持有 Queries 时，Where() 不会把 Queries 转成 Clauses 累积进 whr.Clauses。
func TestWhere_QueriesNotAccumulated(t *testing.T) {
	db := newDB(t)

	whr := where.NewWhere().
		Q("age > ?", 18).
		Q("name LIKE ?", "%alice%")

	require.Equal(t, 0, len(whr.Clauses), "测试前置：Clauses 应为空")

	for i := 0; i < 5; i++ {
		_ = toSQL(db, func(tx *gorm.DB) *gorm.DB {
			return whr.Where(tx).Find(&[]testModel{})
		})
	}

	assert.Equal(t, 0, len(whr.Clauses), "多次调用后 Clauses 仍应为空（Queries 不应被转存为 Clauses）")
}

// ==============================================================
// 二、构造与默认值
// ==============================================================

func TestNewWhere_Defaults(t *testing.T) {
	whr := where.NewWhere()
	assert.Equal(t, 0, whr.Offset)
	assert.Equal(t, -1, whr.Limit, "默认 Limit 应为 -1（GORM 中代表'清除限制'）")
	assert.NotNil(t, whr.Filters)
	assert.Empty(t, whr.Filters)
	assert.NotNil(t, whr.Clauses)
	assert.Empty(t, whr.Clauses)
	assert.NotNil(t, whr.Queries)
	assert.Empty(t, whr.Queries)
	assert.NotNil(t, whr.Selects)
	assert.Empty(t, whr.Selects)
	assert.NotNil(t, whr.Omits)
	assert.Empty(t, whr.Omits)
	assert.NotNil(t, whr.Orders)
	assert.Empty(t, whr.Orders)
	assert.NotNil(t, whr.Joins)
	assert.Empty(t, whr.Joins)
	assert.NotNil(t, whr.Preloads)
	assert.Empty(t, whr.Preloads)
	assert.NotNil(t, whr.Groups)
	assert.Empty(t, whr.Groups)
	assert.NotNil(t, whr.Havings)
	assert.Empty(t, whr.Havings)
}

// ==============================================================
// 三、F 方法的边界
// ==============================================================

// TestF_OddArgsPanics 验证新契约：奇数个参数会 panic（编程错误必须在测试期暴露）。
//
// 设计动机：旧版"静默忽略"会让 F("id", uid, "tenant_id") 这类漏写值的调用
// 整次失效，从而生成"无 WHERE 全表扫描" SQL，是潜在的数据安全事故源。
// panic 化让此类错误在 CI/单测阶段必然暴露。
func TestF_OddArgsPanics(t *testing.T) {
	assert.PanicsWithValue(t,
		"where.Filter: odd number of args (3), must be key-value pairs",
		func() { where.NewWhere().F("id", 1, "name") },
		"奇数个参数应直接 panic，不允许静默忽略",
	)
}

// TestF_NonStringKeyPanics 验证新契约：非 string 类型的 key 会 panic。
//
// 设计动机：map[any]any 中 int(1) 与 int64(1) 是不同的键，会产生难以察觉的
// 键碰撞和"漏过滤"。强制 string 列名能从源头规避这类问题。
func TestF_NonStringKeyPanics(t *testing.T) {
	assert.PanicsWithValue(t,
		"where.Filter: key at index 0 must be string column name, got int",
		func() { where.NewWhere().F(1, "alice") },
		"非 string 类型的 key 应直接 panic",
	)
}

func TestF_NilFiltersAutoInitialized(t *testing.T) {
	whr := &where.Options{}
	require.NotPanics(t, func() {
		whr.F("id", 1)
	}, "F 在 Filters 为 nil 时不应 panic")
	assert.Equal(t, 1, whr.Filters["id"])
}

func TestF_MultiplePairs(t *testing.T) {
	whr := where.NewWhere().F("id", 1, "name", "alice")
	assert.Equal(t, 1, whr.Filters["id"])
	assert.Equal(t, "alice", whr.Filters["name"])
}

// ==============================================================
// 四、Page / Limit / Offset 边界
// ==============================================================

func TestWithPage_PageZeroNormalizedToOne(t *testing.T) {
	whr := where.NewWhere(where.WithPage(0, 10))
	assert.Equal(t, 0, whr.Offset, "page=0 应规范为 page=1，offset=(1-1)*10=0")
	assert.Equal(t, 10, whr.Limit)
}

func TestP_PageZeroNormalizedToOne(t *testing.T) {
	whr := where.P(0, 20)
	assert.Equal(t, 0, whr.Offset)
	assert.Equal(t, 20, whr.Limit)
}

func TestL_NonPositiveNormalizedToDefault(t *testing.T) {
	assert.Equal(t, -1, where.L(0).Limit)
	assert.Equal(t, -1, where.L(-100).Limit)
}

func TestO_NegativeNormalizedToZero(t *testing.T) {
	assert.Equal(t, 0, where.O(-5).Offset)
}

func TestWithLimit_NonPositiveNormalized(t *testing.T) {
	assert.Equal(t, -1, where.NewWhere(where.WithLimit(0)).Limit)
	assert.Equal(t, -1, where.NewWhere(where.WithLimit(-9)).Limit)
}

func TestWithOffset_NegativeNormalized(t *testing.T) {
	assert.Equal(t, 0, where.NewWhere(where.WithOffset(-3)).Offset)
}

// ==============================================================
// 五、SQL 生成行为：综合下推
// ==============================================================

func TestWhere_AppliesFilterAndQueryAndPagination(t *testing.T) {
	db := newDB(t)
	whr := where.NewWhere().
		F("name", "alice").
		Q("age > ?", 18).
		L(10).O(20)

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	upper := strings.ToUpper(sql)

	assert.Contains(t, upper, "WHERE")
	assert.Contains(t, sql, "name")
	assert.Contains(t, sql, "age >")
	assert.Contains(t, upper, "LIMIT 10")
	assert.Contains(t, upper, "OFFSET 20")
}

func TestWhere_SelectsAppliedToFind(t *testing.T) {
	db := newDB(t)
	whr := where.Select("id", "name")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})

	assert.True(t,
		strings.Contains(sql, "`id`,`name`") ||
			strings.Contains(sql, "id`,`name") ||
			strings.Contains(sql, "id\",\"name") ||
			strings.Contains(sql, "id,name"),
		"应该只 SELECT 指定列，实际 SQL: %s", sql)
	assert.NotContains(t, sql, "SELECT *", "Selects 设置后不应是 SELECT *")
}

func TestWhere_OmitsAppliedToFind(t *testing.T) {
	db := newDB(t)
	whr := where.Omit("password")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})

	assert.NotContains(t, sql, "password", "Omit 列不应出现在 SELECT 列表中。SQL: %s", sql)
}

func TestWhere_MultipleOrdersAppliedInSequence(t *testing.T) {
	db := newDB(t)
	whr := where.Order("created_at desc").Order("id asc")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})

	assert.Contains(t, strings.ToUpper(sql), "ORDER BY")
	idxCreated := strings.Index(sql, "created_at desc")
	idxIDAsc := strings.Index(sql, "id asc")
	require.Greaterf(t, idxCreated, 0, "应包含 'created_at desc'，SQL: %s", sql)
	require.Greaterf(t, idxIDAsc, idxCreated, "应按追加顺序排序：'id asc' 必须出现在 'created_at desc' 之后，SQL: %s", sql)
}

// TestWhere_OffsetNotPushedWhenZero 验证 offset=0 时不下推 Offset 子句，
// 避免在 update/delete 等无意义场景生成 OFFSET 0。
func TestWhere_OffsetNotPushedWhenZero(t *testing.T) {
	db := newDB(t)
	whr := where.F("id", 1)

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	assert.NotContains(t, strings.ToUpper(sql), "OFFSET")
}

// ==============================================================
// 六、CountOptions
// ==============================================================

func TestCountOptions_StripsListOnlyFields(t *testing.T) {
	whr := where.NewWhere().
		Filter("name", "alice").
		Query("age > ?", 18).
		Select("id", "name").
		Omit("password").
		Order("id desc").
		L(10).O(20)

	co := whr.CountOptions()
	assert.Equal(t, 0, co.Offset)
	assert.Equal(t, -1, co.Limit)
	assert.Empty(t, co.Selects)
	assert.Empty(t, co.Omits)
	assert.Empty(t, co.Orders)
	assert.Equal(t, "alice", co.Filters["name"])
	assert.Len(t, co.Queries, 1)
}

func TestCountOptions_FiltersShareUnderlyingMap(t *testing.T) {
	whr := where.NewWhere().F("id", 1)
	co := whr.CountOptions()
	whr.Filters["new"] = "value"
	assert.Equal(t, "value", co.Filters["new"], "CountOptions 浅拷贝应共享 Filters map")
}

// ==============================================================
// 七、Tenant
// ==============================================================

func TestRegisterTenant_AppliesValueFromCtx(t *testing.T) {
	where.RegisterTenant("user_id", func(ctx context.Context) string {
		return "tenant-42"
	})
	t.Cleanup(func() { where.RegisterTenant("", nil) })

	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "ignored")

	whr := where.T(ctx)
	assert.Equal(t, "tenant-42", whr.Filters["user_id"])
}

func TestT_NotRegistered_NoPanic(t *testing.T) {
	where.RegisterTenant("", nil)

	require.NotPanics(t, func() {
		whr := where.T(context.Background())
		assert.NotNil(t, whr)
		assert.Empty(t, whr.Filters)
	})
}

// TestRegisterTenant_ConcurrentRaceFree 验证 RegisterTenant 与 T(ctx) 在
// 高并发场景下无数据竞态（搭配 `go test -race` 运行）。
//
// 设计动机：旧实现 `var registeredTenant TenantSpec` 是无保护的全局变量，
// 在生产环境对租户配置做热更新时会触发数据竞争。新实现使用
// atomic.Pointer 保证读写无锁同步。
func TestRegisterTenant_ConcurrentRaceFree(t *testing.T) {
	t.Cleanup(func() { where.RegisterTenant("", nil) })

	const goroutines = 32
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				where.RegisterTenant(
					"user_id",
					func(ctx context.Context) string { return "tenant" },
				)
			}
		}(i)

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = where.T(context.Background())
			}
		}()
	}

	wg.Wait()
}

// ==============================================================
// 八、Option 函数式风格
// ==============================================================

func TestNewWhere_OptionFunctions(t *testing.T) {
	whr := where.NewWhere(
		where.WithOffset(5),
		where.WithLimit(10),
		where.WithFilter(map[any]any{"id": 1}),
		where.WithQuery("age > ?", 18),
		where.WithSelect("id", "name"),
		where.WithOmit("password"),
		where.WithOrder("created_at desc"),
		where.WithJoin("LEFT JOIN posts ON posts.user_id = test_models.id"),
		where.WithPreload("Posts", "published = ?", true),
		where.WithGroup("name"),
		where.WithHaving("COUNT(*) > ?", 1),
	)

	assert.Equal(t, 5, whr.Offset)
	assert.Equal(t, 10, whr.Limit)
	assert.Equal(t, 1, whr.Filters["id"])
	assert.Len(t, whr.Queries, 1)
	assert.Equal(t, []string{"id", "name"}, whr.Selects)
	assert.Equal(t, []string{"password"}, whr.Omits)
	assert.Equal(t, []string{"created_at desc"}, whr.Orders)
	assert.Len(t, whr.Joins, 1)
	assert.Equal(t, "LEFT JOIN posts ON posts.user_id = test_models.id", whr.Joins[0].Query)
	assert.Len(t, whr.Preloads, 1)
	assert.Equal(t, "Posts", whr.Preloads[0].Association)
	assert.Equal(t, []string{"name"}, whr.Groups)
	assert.Len(t, whr.Havings, 1)
	assert.Equal(t, "COUNT(*) > ?", whr.Havings[0].Query)
}

// ==============================================================
// 九、链式 + 包级便捷函数等价
// ==============================================================

func TestPackageLevelHelpers_EquivalentToChained(t *testing.T) {
	chained := where.NewWhere().O(5).L(10).F("id", 1)
	pkg := where.O(5).L(10).F("id", 1)

	assert.Equal(t, chained.Offset, pkg.Offset)
	assert.Equal(t, chained.Limit, pkg.Limit)
	assert.Equal(t, chained.Filters["id"], pkg.Filters["id"])
}

// ==============================================================
// 十、高级 API：JOIN / GROUP / HAVING / PRELOAD
// ==============================================================

func TestWhere_JoinsAppliedInSQL(t *testing.T) {
	db := newDB(t)
	whr := where.Join("LEFT JOIN posts ON posts.user_id = test_models.id")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})

	assert.Contains(t, sql, "LEFT JOIN posts ON posts.user_id = test_models.id",
		"JOIN 子句应被下推到 SQL，实际：%s", sql)
}

func TestWhere_JoinWithArgs(t *testing.T) {
	db := newDB(t)
	whr := where.Join("LEFT JOIN posts ON posts.user_id = test_models.id AND posts.published = ?", true)

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})

	assert.Contains(t, sql, "LEFT JOIN posts")
	assert.Contains(t, sql, "published = ")
}

func TestWhere_MultipleJoinsAppliedInSequence(t *testing.T) {
	db := newDB(t)
	whr := where.
		Join("LEFT JOIN posts ON posts.user_id = test_models.id").
		Join("LEFT JOIN comments ON comments.user_id = test_models.id")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})

	idxPosts := strings.Index(sql, "LEFT JOIN posts")
	idxComments := strings.Index(sql, "LEFT JOIN comments")
	require.Greaterf(t, idxPosts, 0, "应包含第一个 JOIN，SQL: %s", sql)
	require.Greaterf(t, idxComments, idxPosts, "第二个 JOIN 应在第一个之后，SQL: %s", sql)
}

func TestWhere_GroupsAppliedInSQL(t *testing.T) {
	db := newDB(t)
	whr := where.Group("name").Group("age")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	upper := strings.ToUpper(sql)

	assert.Contains(t, upper, "GROUP BY")
	assert.Contains(t, sql, "name")
	assert.Contains(t, sql, "age")
}

func TestWhere_HavingsAppliedInSQL(t *testing.T) {
	db := newDB(t)
	whr := where.Group("name").Having("COUNT(*) > ?", 1)

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	upper := strings.ToUpper(sql)

	assert.Contains(t, upper, "GROUP BY")
	assert.Contains(t, upper, "HAVING")
	assert.Contains(t, sql, "COUNT(*) >")
}

func TestWhere_GroupBeforeHavingInSQL(t *testing.T) {
	db := newDB(t)
	whr := where.Having("SUM(age) > ?", 100).Group("name")

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	upper := strings.ToUpper(sql)

	idxGroup := strings.Index(upper, "GROUP BY")
	idxHaving := strings.Index(upper, "HAVING")
	require.Greaterf(t, idxGroup, 0, "应包含 GROUP BY，SQL: %s", sql)
	require.Greaterf(t, idxHaving, idxGroup, "HAVING 必须出现在 GROUP BY 之后（SQL 语义），SQL: %s", sql)
}

func TestPl_StoresPreloadCorrectly(t *testing.T) {
	whr := where.Preload("Posts", "published = ?", true)

	require.Len(t, whr.Preloads, 1)
	assert.Equal(t, "Posts", whr.Preloads[0].Association)
	require.Len(t, whr.Preloads[0].Args, 2)
	assert.Equal(t, "published = ?", whr.Preloads[0].Args[0])
	assert.Equal(t, true, whr.Preloads[0].Args[1])
}

func TestPl_SimpleWithoutArgs(t *testing.T) {
	whr := where.Preload("Posts")

	require.Len(t, whr.Preloads, 1)
	assert.Equal(t, "Posts", whr.Preloads[0].Association)
	assert.Empty(t, whr.Preloads[0].Args)
}

func TestPl_WithFunctionCondition(t *testing.T) {
	cond := func(db *gorm.DB) *gorm.DB { return db.Order("id desc").Limit(5) }
	whr := where.Preload("Posts", cond)

	require.Len(t, whr.Preloads, 1)
	require.Len(t, whr.Preloads[0].Args, 1)
	assert.NotNil(t, whr.Preloads[0].Args[0], "函数式条件应作为 args 直接保留")
}

func TestPl_MultiplePreloadsAccumulated(t *testing.T) {
	whr := where.Preload("Posts").Preload("Comments", "approved = ?", true)

	require.Len(t, whr.Preloads, 2)
	assert.Equal(t, "Posts", whr.Preloads[0].Association)
	assert.Equal(t, "Comments", whr.Preloads[1].Association)
}

// TestWhere_PreloadsRecordedInGormStatement 验证 Preload 的确被下推到 GORM 内部状态。
func TestWhere_PreloadsRecordedInGormStatement(t *testing.T) {
	db := newDB(t)
	whr := where.Preload("Posts").Preload("Comments", "approved = ?", true)

	session := db.Session(&gorm.Session{})
	result := whr.Where(session)
	require.NotNil(t, result.Statement)
	require.NotNil(t, result.Statement.Preloads)
	assert.Contains(t, result.Statement.Preloads, "Posts")
	assert.Contains(t, result.Statement.Preloads, "Comments")
}

// ==============================================================
// 十一、CountOptions 与高级字段交互
// ==============================================================

func TestCountOptions_KeepsJoinsGroupsHavings(t *testing.T) {
	whr := where.NewWhere().
		Join("LEFT JOIN posts ON posts.user_id = test_models.id").
		Group("name").
		Having("COUNT(*) > ?", 1)

	co := whr.CountOptions()
	assert.Len(t, co.Joins, 1, "Joins 影响 COUNT 行数，必须保留")
	assert.Len(t, co.Groups, 1, "Groups 影响 COUNT，必须保留")
	assert.Len(t, co.Havings, 1, "Havings 影响 COUNT，必须保留")
}

func TestCountOptions_StripsPreloads(t *testing.T) {
	whr := where.NewWhere().Preload("Posts").Preload("Comments")

	co := whr.CountOptions()
	assert.Empty(t, co.Preloads, "Preloads 不影响主表 COUNT，应被剥离")
}

// ==============================================================
// 十二、链式 + 高级 API 综合
// ==============================================================

// ==============================================================
// 十三、命名一致性
// ==============================================================

// TestLegacyShortAliasesEquivalentToLongNames 验证保留的"历史简写"
// （O/L/P/F/C/Q/T）与对应完整命名（无对等长名时仅保留简写）等价。
//
// 注意：新增的 Select/Omit/Order/Join/Preload/Group/Having 不再保留简写别名，
// 仅保留完整命名以减少使用者与维护者的心智负担。
func TestLegacyShortAliasesEquivalentToLongNames(t *testing.T) {
	cond := clause.OrderBy{Columns: []clause.OrderByColumn{
		{Column: clause.Column{Name: "id"}, Desc: true},
	}}

	long := where.NewWhere().
		Page(2, 10).
		Filter("id", 1).
		Clause(cond).
		Query("age > ?", 18)

	short := where.NewWhere().
		P(2, 10).
		F("id", 1).
		C(cond).
		Q("age > ?", 18)

	assert.Equal(t, long.Offset, short.Offset)
	assert.Equal(t, long.Limit, short.Limit)
	assert.Equal(t, long.Filters, short.Filters)
	assert.Equal(t, long.Clauses, short.Clauses)
	assert.Equal(t, long.Queries, short.Queries)
}

// TestPackageLevelFunctions_FullNames 验证所有不与包级类型冲突的便捷函数
// 均提供完整命名，且能正确构造 Options。
func TestPackageLevelFunctions_FullNames(t *testing.T) {
	assert.Equal(t, 5, where.Offset(5).Offset)
	assert.Equal(t, 10, where.Limit(10).Limit)
	assert.Equal(t, 20, where.Page(2, 20).Limit)
	assert.Contains(t, where.Filter("id", 1).Filters, "id")
	assert.Equal(t, []string{"id"}, where.Select("id").Selects)
	assert.Equal(t, []string{"password"}, where.Omit("password").Omits)
	assert.Equal(t, []string{"id desc"}, where.Order("id desc").Orders)
	assert.Equal(t, []string{"name"}, where.Group("name").Groups)
	assert.Len(t, where.Having("COUNT(*) > ?", 1).Havings, 1)
}

// TestPackageLevelFunctions_SQLAlignedNames 验证所有与 SQL 关键字对齐的
// 包级便捷函数（Where/Query/Tenant/Join/Preload/Group/Having 等）均可用。
//
// 历史简写 Q/T/J/Pl 仍保留为向后兼容别名（在历史简写测试中验证）。
func TestPackageLevelFunctions_SQLAlignedNames(t *testing.T) {
	q := where.Query("age > ?", 18)
	assert.Len(t, q.Queries, 1)

	j := where.Join("LEFT JOIN posts ON posts.user_id = test_models.id")
	assert.Len(t, j.Joins, 1)

	p := where.Preload("Posts", "published = ?", true)
	require.Len(t, p.Preloads, 1)
	assert.Equal(t, "Posts", p.Preloads[0].Association)

	w := where.Filter("id", 1)
	assert.Equal(t, 1, w.Filters["id"])
}

// ==============================================================
// 十四、综合用例
// ==============================================================

func TestWhere_ComplexQueryAllFeatures(t *testing.T) {
	db := newDB(t)
	whr := where.NewWhere().
		Filter("name", "alice").
		Query("age > ?", 18).
		Join("LEFT JOIN posts ON posts.user_id = test_models.id").
		Group("name").
		Having("COUNT(*) > ?", 1).
		Select("name", "age").
		Order("name asc").
		L(20).O(40)

	sql := toSQL(db, func(tx *gorm.DB) *gorm.DB {
		return whr.Where(tx).Find(&[]testModel{})
	})
	upper := strings.ToUpper(sql)

	assert.Contains(t, sql, "LEFT JOIN posts")
	assert.Contains(t, upper, "WHERE")
	assert.Contains(t, sql, "name")
	assert.Contains(t, sql, "age >")
	assert.Contains(t, upper, "GROUP BY")
	assert.Contains(t, upper, "HAVING")
	assert.Contains(t, upper, "ORDER BY")
	assert.Contains(t, upper, "LIMIT 20")
	assert.Contains(t, upper, "OFFSET 40")

	idxJoin := strings.Index(upper, "JOIN")
	idxWhere := strings.Index(upper, "WHERE")
	idxGroup := strings.Index(upper, "GROUP BY")
	idxHaving := strings.Index(upper, "HAVING")
	idxOrder := strings.Index(upper, "ORDER BY")
	idxLimit := strings.Index(upper, "LIMIT")

	assert.Less(t, idxJoin, idxWhere, "JOIN 必须先于 WHERE")
	assert.Less(t, idxWhere, idxGroup, "WHERE 必须先于 GROUP BY")
	assert.Less(t, idxGroup, idxHaving, "GROUP BY 必须先于 HAVING")
	assert.Less(t, idxHaving, idxOrder, "HAVING 必须先于 ORDER BY")
	assert.Less(t, idxOrder, idxLimit, "ORDER BY 必须先于 LIMIT")
}
