package store

import (
	"context"
	"errors"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/clin211/linhub/store/logger/empty"
	"github.com/clin211/linhub/store/where"
)

// txKey 是把当前事务存入 context 的私有 key。
type txKey struct{}

// WithTx 把事务 *gorm.DB 注入到 context 中，供下游 Store 使用。
// 当 context 中存在事务时，Store 的所有操作会复用该事务而不是默认连接。
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// TxFromContext 从 context 中提取事务 *gorm.DB（若存在）。
func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	return tx, ok && tx != nil
}

// DBProvider 定义一个用于提供数据库连接的接口。
type DBProvider interface {
	// DB 返回给定上下文的数据库实例，可选地按一组 where.Where 进一步应用条件。
	DB(ctx context.Context, wheres ...where.Where) *gorm.DB
}

// TxRunner 定义跨多个 Store 的事务执行能力。
// 业务层应使用 Run 包装一组 store 操作，确保原子性。
type TxRunner struct {
	root *gorm.DB
}

// NewTxRunner 创建事务执行器。root 必须是非事务的 *gorm.DB（即 raw db）。
func NewTxRunner(root *gorm.DB) *TxRunner {
	return &TxRunner{root: root}
}

// Run 在事务中执行 fn。fn 中通过 WithTx 注入到 ctx 后传给下游 Store，
// 即可让所有 store 操作共享同一事务。
//
// 健壮性保证：
//   - 若 fn 返回 error，事务回滚；
//   - 若 fn panic，GORM 的 Transaction 会通过内部 panicked 标志触发 rollback，
//     保证不会留下"半提交"的脏数据；panic 会原样向上传递以保留调用栈，
//     由调用方自行决定是否恢复。`TestStore_TxRunner_RollbacksOnPanic` 固化此行为；
//   - 若 r 或 r.root 为 nil，直接返回配置错误，不会尝试执行 fn。
//
// 用法：
//
//	runner.Run(ctx, func(ctx context.Context) error {
//	    if err := userStore.Create(ctx, &user); err != nil { return err }
//	    if err := postStore.Create(ctx, &post); err != nil { return err }
//	    return nil
//	})
func (r *TxRunner) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	if r == nil || r.root == nil {
		return errors.New("tx runner is not configured")
	}
	return r.root.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := WithTx(ctx, tx)
		return fn(txCtx)
	})
}

// Store 表示一个具有日志功能的通用数据存储。
type Store[T any] struct {
	logger  Logger
	storage DBProvider
}

// NewStore 使用提供的 DBProvider 与 Logger 创建一个新的 Store 实例。
//
// logger 传入 nil 时会自动使用 empty.NewLogger() 作为占位实现。
func NewStore[T any](storage DBProvider, logger Logger) *Store[T] {
	if logger == nil {
		logger = empty.NewLogger()
	}

	return &Store[T]{
		logger:  logger,
		storage: storage,
	}
}

// raw 返回未叠加 where 条件的原始 *gorm.DB。
//
// 若 context 中存在事务（通过 WithTx 注入），优先返回事务句柄；
// 否则向 storage 索取一个新的 db 实例。
//
// 设计动机：把"事务提取"的逻辑收拢到唯一来源，避免 db()/Count()/List()
// 多处重复造成事务参与不一致（曾在 List 中遗漏导致事务失效）。
func (s *Store[T]) raw(ctx context.Context) *gorm.DB {
	if tx, ok := TxFromContext(ctx); ok {
		return tx
	}
	return s.storage.DB(ctx)
}

// db 获取数据库实例并应用提供的 where 条件。
// 若 context 中存在事务（通过 WithTx 注入），优先使用事务句柄。
func (s *Store[T]) db(ctx context.Context, wheres ...where.Where) *gorm.DB {
	dbInstance := s.raw(ctx)
	for _, whr := range wheres {
		if whr != nil {
			dbInstance = whr.Where(dbInstance)
		}
	}
	return dbInstance
}

// Create 向数据库中插入一个新对象。
func (s *Store[T]) Create(ctx context.Context, obj *T) error {
	if err := s.db(ctx).Create(obj).Error; err != nil {
		s.logger.Error(ctx, err, "Failed to insert object into database", "object", obj)
		return err
	}
	return nil
}

// Update 修改数据库中的现有对象（全字段保存，对应 GORM `Save`）。
//
// 行为细节：
//   - Save 会写入对象的所有字段（包括零值）。如果只想更新部分字段，请使用：
//   - UpdatePartial：用 map 描述要更新的列，最干净；
//   - UpdateColumns：保留结构体形参但仅更新指定列。
//   - Save 会触发 GORM 的 BeforeSave/BeforeUpdate/AfterUpdate/AfterSave 钩子；
//     若模型实现了相关接口（如 BeforeUpdate(tx *gorm.DB) error），钩子内的 error
//     会作为本方法的返回 error 向上传递。
//   - 若主键为零值，Save 会退化为 Insert（GORM 的设计语义）。调用方需自行确保
//     传入的 obj 至少持有有效主键。
func (s *Store[T]) Update(ctx context.Context, obj *T) error {
	if err := s.db(ctx).Save(obj).Error; err != nil {
		s.logger.Error(ctx, err, "Failed to update object in database", "object", obj)
		return err
	}
	return nil
}

// UpdatePartial 在 opts 指定的过滤条件下，仅更新 attrs 中给出的列。
//
// 使用 map 形式的好处是 GORM 不会忽略零值（与 Updates(struct) 不同），
// 适合"显式将某字段置 0 / 空字符串"的场景。
//
// 返回值：受影响的行数与错误。
//
// 示例：
//
//	rows, err := userStore.UpdatePartial(ctx, where.F("id", uid),
//	    map[string]any{"nickname": "alice", "email": "a@b.c"})
func (s *Store[T]) UpdatePartial(ctx context.Context, opts *where.Options, attrs map[string]any) (int64, error) {
	if len(attrs) == 0 {
		return 0, nil
	}
	res := s.db(ctx, opts).Model(new(T)).Updates(attrs)
	if res.Error != nil {
		s.logger.Error(ctx, res.Error, "Failed to partial-update object", "conditions", opts, "attrs", attrs)
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// UpdateColumns 在 opts 指定的过滤条件下，仅更新 obj 中 columns 列对应的值。
//
// 与 UpdatePartial 不同，本方法接收结构体形参，由 GORM 的 Select 控制实际写入的列。
// 适合调用方已经持有结构体、且不希望额外构造 map 的场景。
//
// 当 columns 为空时本方法不会做任何更新（防止误用导致全字段覆盖）。
//
// 返回值：受影响的行数与错误。
//
// 示例：
//
//	rows, err := userStore.UpdateColumns(ctx, where.F("id", uid), userM, "nickname", "email")
func (s *Store[T]) UpdateColumns(ctx context.Context, opts *where.Options, obj *T, columns ...string) (int64, error) {
	if len(columns) == 0 {
		return 0, nil
	}
	res := s.db(ctx, opts).Model(obj).Select(columns).Updates(obj)
	if res.Error != nil {
		s.logger.Error(ctx, res.Error, "Failed to update columns", "conditions", opts, "columns", columns)
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// Delete 根据提供的 where 选项从数据库中删除对象，返回受影响的行数。
//
// 设计说明：
//   - GORM 的 Delete 在"无匹配行"时返回 nil（不会返回 ErrRecordNotFound），
//     因此调用方不能用 error 是否为 nil 来判断"是否真的删除了某行"；
//   - 本方法把 RowsAffected 暴露给调用方，由业务层自决"删除 0 行"是否算异常；
//   - 这是相对旧实现 (`Delete(...) error`) 的 breaking change，但能从源头消除
//     "误删 0 行被静默吞掉"的隐患。
func (s *Store[T]) Delete(ctx context.Context, opts *where.Options) (int64, error) {
	res := s.db(ctx, opts).Delete(new(T))
	if res.Error != nil {
		s.logger.Error(ctx, res.Error, "Failed to delete object from database", "conditions", opts)
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// Get 根据提供的 where 选项从数据库中获取单个对象（对应 GORM `First`）。
//
// 错误语义：
//   - 未找到记录时返回 (nil, gorm.ErrRecordNotFound)；调用方可使用
//     errors.Is(err, gorm.ErrRecordNotFound) 区分"未找到"与"DB 错误"。
//   - 其他底层错误（连接、SQL 语法等）按 GORM 原样返回。
//   - 注意：本方法会按主键 ASC 排序后取第一条。如需自定义排序，请在 opts 中
//     通过 Order(...) 指定，避免依赖隐式排序。
func (s *Store[T]) Get(ctx context.Context, opts *where.Options) (*T, error) {
	var obj T
	if err := s.db(ctx, opts).First(&obj).Error; err != nil {
		s.logger.Error(ctx, err, "Failed to retrieve object from database", "conditions", opts)
		return nil, err
	}
	return &obj, nil
}

// Count 根据提供的 where 选项返回匹配的记录数（不应用分页/排序/Select）。
func (s *Store[T]) Count(ctx context.Context, opts *where.Options) (int64, error) {
	q := s.raw(ctx).Model(new(T))
	if opts != nil {
		q = opts.CountOptions().Where(q)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		s.logger.Error(ctx, err, "Failed to count objects from database", "conditions", opts)
		return 0, err
	}
	return count, nil
}

// Exists 根据 where 选项判断是否存在至少一条记录。
func (s *Store[T]) Exists(ctx context.Context, opts *where.Options) (bool, error) {
	count, err := s.Count(ctx, opts)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// List 根据提供的 where 选项从数据库中获取对象列表。
//
// 实现要点：
//   - 优先复用 ctx 中的事务句柄（修复历史 Bug：旧实现绕过 tx，导致事务内 List
//     读取非事务连接，破坏读自身写入的事务语义）。
//   - 先 Count 总数（不应用 offset/limit/order），再 Find 分页结果。
//   - 当 opts 设置了 Orders 时使用其指定的排序；否则尝试推导主键列名作为
//     默认 ORDER BY，推导失败则回退到 "id desc"（向后兼容）。
func (s *Store[T]) List(ctx context.Context, opts *where.Options) (count int64, ret []*T, err error) {
	dbInstance := s.raw(ctx)

	countDB := dbInstance.Model(new(T))
	if opts != nil {
		countDB = opts.CountOptions().Where(countDB)
	}
	if err = countDB.Count(&count).Error; err != nil {
		s.logger.Error(ctx, err, "Failed to count objects from database", "conditions", opts)
		return 0, nil, err
	}

	listDB := dbInstance
	if opts != nil {
		listDB = opts.Where(listDB)
	}
	if opts == nil || len(opts.Orders) == 0 {
		listDB = listDB.Order(defaultOrder[T]())
	}
	if err = listDB.Find(&ret).Error; err != nil {
		s.logger.Error(ctx, err, "Failed to list objects from database", "conditions", opts)
		return 0, nil, err
	}

	return count, ret, nil
}

// defaultOrder 推导模型的默认 ORDER BY 表达式。
//
// 优先使用主键列名 + " desc"（兼容 UUID/雪花 ID 等非 id 主键模型）；
// 推导失败时回退到 "id desc" 以保持向后兼容。
//
// 通过 GORM 的 schema 缓存（schema.Parse 内部缓存到 sync.Map），
// 同一类型 T 仅在首次调用时支付解析成本。
func defaultOrder[T any]() string {
	const fallback = "id desc"

	s, err := schema.Parse(new(T), &schemaCacheStore, schema.NamingStrategy{})
	if err != nil || s == nil || s.PrioritizedPrimaryField == nil {
		return fallback
	}
	pk := s.PrioritizedPrimaryField.DBName
	if pk == "" {
		return fallback
	}
	return pk + " desc"
}

// schemaCacheStore 是 schema.Parse 所需的缓存容器。
// schema.Parse 要求传入一个 *sync.Map（非 nil），用于缓存已解析的 schema。
var schemaCacheStore sync.Map
