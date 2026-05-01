package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/clin211/linhub/store"
	"github.com/clin211/linhub/store/logger/empty"
	"github.com/clin211/linhub/store/logger/onex"
	"github.com/clin211/linhub/store/where"
)

// 编译期断言：empty / onex 两个内置 logger 都必须实现 store.Logger 接口。
// 此处会在签名漂移时立即触发编译错误（曾经的 onex.Error 缺 ctx 即属此类问题）。
var (
	_ store.Logger = empty.NewLogger()
	_ store.Logger = onex.NewLogger()
)

// testUser 是仅供本测试包使用的轻量模型，覆盖足够多的列以验证部分更新行为。
type testUser struct {
	ID       int    `gorm:"primaryKey"`
	Name     string `gorm:"size:64"`
	Email    string `gorm:"size:128"`
	Age      int
	Status   string `gorm:"size:32"`
	Password string `gorm:"size:64"`
}

// fakeDBProvider 实现 store.DBProvider，固定返回同一个 *gorm.DB。
//
// 注意：当前 store.Store.db() 不会向 DBProvider 传 wheres，但保留参数以满足接口签名。
type fakeDBProvider struct {
	db *gorm.DB
}

func (f *fakeDBProvider) DB(_ context.Context, wheres ...where.Where) *gorm.DB {
	db := f.db
	for _, w := range wheres {
		if w != nil {
			db = w.Where(db)
		}
	}
	return db
}

// newStoreTest 启动一个内存 sqlite + 已建表，并返回对应的 *Store[testUser]。
func newStoreTest(t *testing.T) (*store.Store[testUser], *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&testUser{}))
	s := store.NewStore[testUser](&fakeDBProvider{db: db}, nil)
	return s, db
}

// seedUser 是测试辅助：插入一条用户并返回。
func seedUser(t *testing.T, s *store.Store[testUser], u *testUser) *testUser {
	t.Helper()
	require.NoError(t, s.Create(context.Background(), u))
	require.NotZero(t, u.ID)
	return u
}

// ==============================================================
// 一、基础 CRUD 回归
// ==============================================================

func TestStore_Create(t *testing.T) {
	s, _ := newStoreTest(t)
	u := &testUser{Name: "alice", Email: "a@b.c", Age: 20}
	require.NoError(t, s.Create(context.Background(), u))
	assert.NotZero(t, u.ID)
}

func TestStore_GetByFilter(t *testing.T) {
	s, _ := newStoreTest(t)
	u := seedUser(t, s, &testUser{Name: "alice"})

	got, err := s.Get(context.Background(), where.F("id", u.ID))
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Name)
}

func TestStore_Get_RecordNotFound(t *testing.T) {
	s, _ := newStoreTest(t)
	got, err := s.Get(context.Background(), where.F("id", 9999))
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestStore_DeleteByFilter(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	u := seedUser(t, s, &testUser{Name: "alice"})

	rows, err := s.Delete(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows, "应删除 1 行")

	exists, err := s.Exists(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestStore_Delete_NoMatch_ReturnsZeroRows 验证新契约：
// 无匹配行时返回 (0, nil)，让业务层自决是否当作错误处理。
func TestStore_Delete_NoMatch_ReturnsZeroRows(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()

	rows, err := s.Delete(ctx, where.F("id", 999999))
	require.NoError(t, err, "无匹配行不应返回错误（GORM 语义）")
	assert.Equal(t, int64(0), rows, "无匹配行应返回 RowsAffected=0")
}

func TestStore_CountAndExists(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	seedUser(t, s, &testUser{Name: "alice", Status: "active"})
	seedUser(t, s, &testUser{Name: "bob", Status: "active"})
	seedUser(t, s, &testUser{Name: "carol", Status: "inactive"})

	count, err := s.Count(ctx, where.F("status", "active"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	exists, err := s.Exists(ctx, where.F("status", "inactive"))
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestStore_List_DefaultOrder(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	seedUser(t, s, &testUser{Name: "alice"})
	seedUser(t, s, &testUser{Name: "bob"})
	seedUser(t, s, &testUser{Name: "carol"})

	count, list, err := s.List(ctx, where.NewWhere())
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
	require.Len(t, list, 3)
	assert.Equal(t, "carol", list[0].Name, "默认应按 id desc 排序")
}

func TestStore_List_CustomOrder(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	seedUser(t, s, &testUser{Name: "alice"})
	seedUser(t, s, &testUser{Name: "bob"})

	_, list, err := s.List(ctx, where.Order("name asc"))
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "alice", list[0].Name)
}

// ==============================================================
// 二、Update 全字段保存（Save 行为）
// ==============================================================

// TestStore_Update_FullOverwrite 验证 Save 是全字段保存：
// 修改了对象的某个字段后调用 Update，所有字段都会被写入数据库。
func TestStore_Update_FullOverwrite(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()

	u := seedUser(t, s, &testUser{Name: "alice", Email: "a@b.c", Age: 20, Status: "active", Password: "secret"})

	got, err := s.Get(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	got.Email = "new@b.c"
	require.NoError(t, s.Update(ctx, got))

	after, err := s.Get(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	assert.Equal(t, "new@b.c", after.Email)
	assert.Equal(t, "alice", after.Name, "Save 是全字段保存，但本次 got 已包含原值，因此其他字段保留")
	assert.Equal(t, "secret", after.Password)
}

// ==============================================================
// 三、UpdatePartial（map 形式部分更新）
// ==============================================================

// TestStore_UpdatePartial_OnlyUpdatesGivenColumns 是关键测试：
// 验证 UpdatePartial 仅更新 attrs 中给出的列，其他列保持不变。
func TestStore_UpdatePartial_OnlyUpdatesGivenColumns(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	u := seedUser(t, s, &testUser{Name: "alice", Email: "a@b.c", Age: 20, Status: "active", Password: "secret"})

	rows, err := s.UpdatePartial(ctx, where.F("id", u.ID), map[string]any{
		"email": "new@b.c",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, err := s.Get(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Name, "Name 未在 attrs 中，不应改变")
	assert.Equal(t, "new@b.c", got.Email, "Email 在 attrs 中，应被更新")
	assert.Equal(t, 20, got.Age, "Age 未在 attrs 中，不应改变")
	assert.Equal(t, "active", got.Status)
	assert.Equal(t, "secret", got.Password, "Password 未在 attrs 中，不应改变")
}

// TestStore_UpdatePartial_AllowsZeroValues 验证 map 形式能显式写入零值
// （这是相对 Updates(struct) 的优势：Updates(struct) 会跳过零值字段）。
func TestStore_UpdatePartial_AllowsZeroValues(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	u := seedUser(t, s, &testUser{Name: "alice", Age: 30, Status: "active"})

	rows, err := s.UpdatePartial(ctx, where.F("id", u.ID), map[string]any{
		"age":    0,
		"status": "",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, err := s.Get(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	assert.Equal(t, 0, got.Age, "map 应能将 age 显式置为 0")
	assert.Equal(t, "", got.Status, "map 应能将 status 显式置为空字符串")
}

func TestStore_UpdatePartial_EmptyAttrs_NoOp(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	u := seedUser(t, s, &testUser{Name: "alice"})

	rows, err := s.UpdatePartial(ctx, where.F("id", u.ID), map[string]any{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows, "attrs 为空时应返回 0 行受影响且不执行 SQL")

	got, _ := s.Get(ctx, where.F("id", u.ID))
	assert.Equal(t, "alice", got.Name, "数据未被改动")
}

func TestStore_UpdatePartial_NilAttrs_NoOp(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	u := seedUser(t, s, &testUser{Name: "alice"})

	rows, err := s.UpdatePartial(ctx, where.F("id", u.ID), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows, "attrs 为 nil 时应等同于空")
}

// TestStore_UpdatePartial_RespectsCondition 验证 where 条件限定生效，
// 避免误更新未匹配的行。
func TestStore_UpdatePartial_RespectsCondition(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()

	u1 := seedUser(t, s, &testUser{Name: "alice"})
	u2 := seedUser(t, s, &testUser{Name: "bob"})

	rows, err := s.UpdatePartial(ctx, where.F("name", "bob"), map[string]any{"name": "robert"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got1, _ := s.Get(ctx, where.F("id", u1.ID))
	got2, _ := s.Get(ctx, where.F("id", u2.ID))
	assert.Equal(t, "alice", got1.Name, "未匹配的行不应被更新")
	assert.Equal(t, "robert", got2.Name)
}

// TestStore_UpdatePartial_NoMatchingRows 验证条件不匹配时返回 0 且不报错。
func TestStore_UpdatePartial_NoMatchingRows(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	seedUser(t, s, &testUser{Name: "alice"})

	rows, err := s.UpdatePartial(ctx, where.F("id", 9999), map[string]any{"name": "x"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows)
}

// ==============================================================
// 四、UpdateColumns（结构体 + Select 形式部分更新）
// ==============================================================

// TestStore_UpdateColumns_OnlyUpdatesGivenColumns 验证：
// UpdateColumns 仅更新指定的列，即使 obj 中其他字段被修改，也不会写入。
func TestStore_UpdateColumns_OnlyUpdatesGivenColumns(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()

	u := seedUser(t, s, &testUser{Name: "alice", Email: "a@b.c", Age: 20, Status: "active", Password: "secret"})

	u.Name = "bob"
	u.Email = "bob@b.c"
	u.Age = 99
	u.Status = "inactive"
	u.Password = "newsecret"

	rows, err := s.UpdateColumns(ctx, where.F("id", u.ID), u, "name", "email")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got, err := s.Get(ctx, where.F("id", u.ID))
	require.NoError(t, err)
	assert.Equal(t, "bob", got.Name)
	assert.Equal(t, "bob@b.c", got.Email)
	assert.Equal(t, 20, got.Age, "Age 不在 columns 中，不应被更新")
	assert.Equal(t, "active", got.Status)
	assert.Equal(t, "secret", got.Password, "Password 不在 columns 中，不应被更新")
}

func TestStore_UpdateColumns_EmptyColumns_NoOp(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()
	u := seedUser(t, s, &testUser{Name: "alice"})

	u.Name = "bob"
	rows, err := s.UpdateColumns(ctx, where.F("id", u.ID), u)
	require.NoError(t, err)
	assert.Equal(t, int64(0), rows, "columns 为空时应返回 0 且不执行 SQL")

	got, _ := s.Get(ctx, where.F("id", u.ID))
	assert.Equal(t, "alice", got.Name, "数据应保持原状")
}

func TestStore_UpdateColumns_RespectsCondition(t *testing.T) {
	s, _ := newStoreTest(t)
	ctx := context.Background()

	u1 := seedUser(t, s, &testUser{Name: "alice"})
	u2 := seedUser(t, s, &testUser{Name: "bob"})

	u2.Email = "bob@b.c"
	rows, err := s.UpdateColumns(ctx, where.F("id", u2.ID), u2, "email")
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)

	got1, _ := s.Get(ctx, where.F("id", u1.ID))
	got2, _ := s.Get(ctx, where.F("id", u2.ID))
	assert.Equal(t, "", got1.Email)
	assert.Equal(t, "bob@b.c", got2.Email)
}

// ==============================================================
// 五、事务支持
// ==============================================================

// TestStore_TxRunner_Commit 验证事务内多次操作能原子提交。
func TestStore_TxRunner_Commit(t *testing.T) {
	s, db := newStoreTest(t)
	ctx := context.Background()
	runner := store.NewTxRunner(db)

	err := runner.Run(ctx, func(txCtx context.Context) error {
		if err := s.Create(txCtx, &testUser{Name: "alice"}); err != nil {
			return err
		}
		return s.Create(txCtx, &testUser{Name: "bob"})
	})
	require.NoError(t, err)

	count, _ := s.Count(ctx, nil)
	assert.Equal(t, int64(2), count)
}

// TestStore_TxRunner_Rollback 验证事务在错误时回滚。
func TestStore_TxRunner_Rollback(t *testing.T) {
	s, db := newStoreTest(t)
	ctx := context.Background()
	runner := store.NewTxRunner(db)

	expected := errors.New("rollback please")
	err := runner.Run(ctx, func(txCtx context.Context) error {
		if err := s.Create(txCtx, &testUser{Name: "alice"}); err != nil {
			return err
		}
		return expected
	})
	assert.ErrorIs(t, err, expected)

	count, _ := s.Count(ctx, nil)
	assert.Equal(t, int64(0), count, "事务回滚后应没有任何记录")
}

// TestStore_TxRunner_RollbacksOnPanic 固化 GORM Transaction 的 panic 语义：
// fn 内 panic 时，GORM 会触发事务回滚，panic 原样向上抛出。
//
// 设计动机：这是事务安全的关键不变量——业务层任何 panic 都不能留下
// "半提交"的脏数据。本测试既验证当前 GORM 版本的内置保护，也作为升级 GORM
// 时的回归阀门（一旦行为退化，CI 会立即捕获）。
func TestStore_TxRunner_RollbacksOnPanic(t *testing.T) {
	s, db := newStoreTest(t)
	ctx := context.Background()
	runner := store.NewTxRunner(db)

	assert.PanicsWithValue(t, "boom", func() {
		_ = runner.Run(ctx, func(txCtx context.Context) error {
			require.NoError(t, s.Create(txCtx, &testUser{Name: "alice"}))
			panic("boom")
		})
	}, "fn panic 应原样抛出，保留调用栈")

	count, _ := s.Count(ctx, nil)
	assert.Equal(t, int64(0), count, "panic 触发的事务回滚必须保证没有任何记录被持久化")
}

// TestStore_UpdatePartial_InsideTx 验证 UpdatePartial 在事务上下文中能正确复用事务。
func TestStore_UpdatePartial_InsideTx(t *testing.T) {
	s, db := newStoreTest(t)
	ctx := context.Background()
	runner := store.NewTxRunner(db)

	u := seedUser(t, s, &testUser{Name: "alice", Email: "a@b.c"})

	err := runner.Run(ctx, func(txCtx context.Context) error {
		_, err := s.UpdatePartial(txCtx, where.F("id", u.ID), map[string]any{"email": "tx@b.c"})
		return err
	})
	require.NoError(t, err)

	got, _ := s.Get(ctx, where.F("id", u.ID))
	assert.Equal(t, "tx@b.c", got.Email)
}

// TestStore_List_InsideTx 是 P1 Bug 的回归测试：
//
// 修复前：List() 直接调用 s.storage.DB(ctx)，绕过 ctx 中的事务句柄；
// 此时事务内 List 读到的是非事务连接，看不到"事务内已写入但未提交"的数据。
//
// 修复后：List() 复用 raw() 提取事务，应能读到当前事务内的写入。
func TestStore_List_InsideTx(t *testing.T) {
	s, db := newStoreTest(t)
	ctx := context.Background()
	runner := store.NewTxRunner(db)

	err := runner.Run(ctx, func(txCtx context.Context) error {
		if err := s.Create(txCtx, &testUser{Name: "alice"}); err != nil {
			return err
		}
		if err := s.Create(txCtx, &testUser{Name: "bob"}); err != nil {
			return err
		}

		count, list, err := s.List(txCtx, where.NewWhere())
		if err != nil {
			return err
		}
		assert.Equal(t, int64(2), count, "事务内 List 应能看到事务内已写入的 2 行")
		assert.Len(t, list, 2)
		return nil
	})
	require.NoError(t, err)

	count, _, err := s.List(ctx, where.NewWhere())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "事务提交后非事务连接应能看到 2 行")
}

// TestStore_List_InsideTx_RollbackHidesWrites 验证：
// 如果 List 真正参与事务，那么事务回滚后这些写入对外部不可见。
// 这一测试同时覆盖了 List 与 Create 的事务参与一致性。
func TestStore_List_InsideTx_RollbackHidesWrites(t *testing.T) {
	s, db := newStoreTest(t)
	ctx := context.Background()
	runner := store.NewTxRunner(db)

	expected := errors.New("rollback please")
	err := runner.Run(ctx, func(txCtx context.Context) error {
		_ = s.Create(txCtx, &testUser{Name: "alice"})

		count, _, err := s.List(txCtx, where.NewWhere())
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "事务内 List 应看到事务内的 1 行")

		return expected
	})
	assert.ErrorIs(t, err, expected)

	count, _, err := s.List(ctx, where.NewWhere())
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "事务回滚后不应有任何记录")
}

// TestStore_List_DefaultOrder_NonIDPrimaryKey 验证 P2-3 改造：
// 当模型的主键字段不叫 "id" 时，List 默认 ORDER BY 应使用真实的主键列名，
// 而不是硬编码的 "id desc"（旧实现会生成无效 SQL）。
func TestStore_List_DefaultOrder_NonIDPrimaryKey(t *testing.T) {
	type tagModel struct {
		TagID int    `gorm:"primaryKey;column:tag_id"`
		Name  string `gorm:"size:64"`
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&tagModel{}))

	tagStore := store.NewStore[tagModel](&fakeDBProvider{db: db}, nil)
	ctx := context.Background()

	require.NoError(t, tagStore.Create(ctx, &tagModel{Name: "go"}))
	require.NoError(t, tagStore.Create(ctx, &tagModel{Name: "rust"}))

	count, list, err := tagStore.List(ctx, where.NewWhere())
	require.NoError(t, err, "默认 ORDER BY 应使用真实主键列名 tag_id 而非硬编码 id")
	assert.Equal(t, int64(2), count)
	require.Len(t, list, 2)
	assert.Equal(t, "rust", list[0].Name, "默认按主键 desc 排序，最后插入的应排在前")
}
