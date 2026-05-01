package db

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgreSQLOptions 定义 PostgreSQL 数据库的配置选项。
type PostgreSQLOptions struct {
	Addr                  string
	Username              string
	Password              string
	Database              string
	MaxIdleConnections    int
	MaxOpenConnections    int
	MaxConnectionLifeTime time.Duration
	// SSLMode 控制 TLS 行为，默认 "prefer"（生产环境建议 "require" 或 "verify-full"）。
	SSLMode string
	// +optional
	Logger logger.Interface
}

// DSN 根据 PostgreSQLOptions 返回 DSN 连接字符串。
// SSL 模式默认 prefer（优先 TLS，但不强制证书校验），避免 disable 这种不安全的默认值。
func (o *PostgreSQLOptions) DSN() string {
	splited := strings.Split(o.Addr, ":")
	host, port := splited[0], "5432"
	if len(splited) > 1 {
		port = splited[1]
	}

	sslmode := o.SSLMode
	if sslmode == "" {
		sslmode = "prefer"
	}

	return fmt.Sprintf(`user=%s password=%s host=%s port=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai connect_timeout=10`,
		o.Username,
		o.Password,
		host,
		port,
		o.Database,
		sslmode,
	)
}

// NewPostgreSQL 使用给定的配置选项创建一个新的 gorm DB 实例。
func NewPostgreSQL(opts *PostgreSQLOptions) (*gorm.DB, error) {
	// 设置默认值，确保 opts 中的所有字段均可用。
	setPostgreSQLDefaults(opts)

	db, err := gorm.Open(postgres.Open(opts.DSN()), &gorm.Config{
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

// setPostgreSQLDefaults 为部分字段设置可用的默认值（与 MySQL 默认值一致）。
func setPostgreSQLDefaults(opts *PostgreSQLOptions) {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:5432"
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
