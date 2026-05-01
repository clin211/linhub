package db

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQLiteOptions 定义 SQLite 数据库的配置选项。
type SQLiteOptions struct {
	Addr                  string // 对于 SQLite，此字段表示数据库文件路径
	Username              string // SQLite 中未使用，保留是为了与其他数据库保持一致
	Password              string // SQLite 中未使用，保留是为了与其他数据库保持一致
	Database              string // 数据库文件名/路径（SQLite 的主要字段）
	MaxIdleConnections    int
	MaxOpenConnections    int
	MaxConnectionLifeTime time.Duration
	// +optional
	Logger logger.Interface
}

// DSN 根据 SQLiteOptions 返回 DSN 连接字符串。
func (o *SQLiteOptions) DSN() string {
	// 对于 SQLite，我们将 Database 字段作为主要的文件路径
	// 如果 Database 为空，则回退到 Addr 字段
	dsn := o.Database
	if dsn == "" && o.Addr != "" {
		dsn = o.Addr
	}

	// 如果仍然为空，则使用默认值
	if dsn == "" {
		dsn = "app.db"
	}

	return dsn
}

// NewSQLite 使用给定的配置选项创建一个新的 gorm DB 实例。
func NewSQLite(opts *SQLiteOptions) (*gorm.DB, error) {
	// 设置默认值，确保 opts 中的所有字段均可用。
	setSQLiteDefaults(opts)

	db, err := gorm.Open(sqlite.Open(opts.DSN()), &gorm.Config{
		// PrepareStmt 会以缓存的预编译语句方式执行给定的查询。
		// 这可以提升性能。
		PrepareStmt: true,
		Logger:      opts.Logger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SetMaxOpenConns 设置数据库的最大打开连接数。
	sqlDB.SetMaxOpenConns(opts.MaxOpenConnections)

	// SetConnMaxLifetime 设置一个连接可被复用的最长时间。
	sqlDB.SetConnMaxLifetime(opts.MaxConnectionLifeTime)

	// SetMaxIdleConns 设置空闲连接池中的最大连接数。
	sqlDB.SetMaxIdleConns(opts.MaxIdleConnections)

	return db, nil
}

// setSQLiteDefaults 为部分字段设置可用的默认值。
func setSQLiteDefaults(opts *SQLiteOptions) {
	// 为了与 PostgreSQL 保持一致，先检查 Addr，再检查 Database
	if opts.Addr == "" && opts.Database == "" {
		opts.Database = "app.db" // 默认的 SQLite 数据库文件
	}

	// 由于文件锁的限制，SQLite 在使用较少连接时通常表现更好
	if opts.MaxIdleConnections == 0 {
		opts.MaxIdleConnections = 2 // SQLite 的保守取值
	}
	if opts.MaxOpenConnections == 0 {
		opts.MaxOpenConnections = 5 // 限制连接数以避免锁竞争问题
	}
	if opts.MaxConnectionLifeTime == 0 {
		opts.MaxConnectionLifeTime = time.Duration(10) * time.Second // 与 PostgreSQL 相同
	}
	if opts.Logger == nil {
		opts.Logger = logger.Default
	}
}

// NewInMemorySQLite 创建一个新的内存型 SQLite 数据库实例。
// 此函数适用于测试或临时数据存储场景。
func NewInMemorySQLite(dbFile string) (*gorm.DB, error) {
	opts := &SQLiteOptions{
		// 使用 SQLite 内存模式配置数据库
		// ?cache=shared 用于将 SQLite 的缓存模式设置为共享缓存模式。
		// 默认情况下，每个 SQLite 数据库连接都有自己的私有缓存，这种模式称为私有缓存。
		// 使用共享缓存模式允许不同连接共享同一个内存数据库及其缓存。
		Database:              fmt.Sprintf("file:%s?cache=shared", dbFile),
		MaxIdleConnections:    1,
		MaxOpenConnections:    1,
		MaxConnectionLifeTime: time.Hour,
		Logger:                logger.Default,
	}

	return NewSQLite(opts)
}
