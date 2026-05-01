package db

import (
	"fmt"
	"time"

	"database/sql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MySQLOptions 定义 MySQL 数据库的配置选项。
type MySQLOptions struct {
	Addr                  string
	Username              string
	Password              string
	Database              string
	MaxIdleConnections    int
	MaxOpenConnections    int
	MaxConnectionLifeTime time.Duration
	// +optional
	Logger logger.Interface
}

// DSN 根据 MySQLOptions 返回 DSN 连接字符串。
// charset 使用 utf8mb4 以支持完整 Unicode（包括 emoji 与稀有汉字）。
// 默认带连接超时，防止应用启动时无限阻塞。
func (o *MySQLOptions) DSN() string {
	return fmt.Sprintf(`%s:%s@tcp(%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=%t&loc=%s&timeout=10s&readTimeout=30s&writeTimeout=30s`,
		o.Username,
		o.Password,
		o.Addr,
		o.Database,
		true,
		"Local")
}

// NewMySQL 使用给定的配置选项创建一个新的 gorm DB 实例。
func NewMySQL(opts *MySQLOptions) (*gorm.DB, error) {
	// 设置默认值，确保 opts 中的所有字段均可用。
	setMySQLDefaults(opts)

	db, err := gorm.Open(mysql.Open(opts.DSN()), &gorm.Config{
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

// setMySQLDefaults 为部分字段设置可用的默认值。
// 默认值针对单个微服务实例的合理资源占用：
//   - MaxOpenConnections=20：避免大量副本耗尽数据库连接上限
//   - MaxIdleConnections=10：保持热连接，减少冷启动延迟
//   - MaxConnectionLifeTime=30min：兼顾长连接复用与连接刷新
func setMySQLDefaults(opts *MySQLOptions) {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:3306"
	}
	if opts.MaxIdleConnections == 0 {
		opts.MaxIdleConnections = 10
	}
	if opts.MaxOpenConnections == 0 {
		opts.MaxOpenConnections = 20
	}
	if opts.MaxConnectionLifeTime == 0 {
		opts.MaxConnectionLifeTime = 30 * time.Minute
	}
	if opts.Logger == nil {
		opts.Logger = logger.Default
	}
}

func MustRawDB(db *gorm.DB) *sql.DB {
	raw, err := db.DB()
	if err != nil {
		panic(err)
	}
	return raw
}
