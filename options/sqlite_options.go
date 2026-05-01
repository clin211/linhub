package options

import (
	"time"

	"github.com/spf13/pflag"
	"gorm.io/gorm"

	"github.com/clin211/linhub/db"
	"github.com/clin211/linhub/log"
)

var _ IOptions = (*SQLiteOptions)(nil)

// SQLiteOptions 定义 SQLite 数据库的选项。
type SQLiteOptions struct {
	Addr                  string        `json:"addr,omitempty" mapstructure:"addr"`         // SQLite 数据库文件路径
	Username              string        `json:"username,omitempty" mapstructure:"username"` // 为接口兼容性保留的字段
	Password              string        `json:"-" mapstructure:"password"`                  // 保留字段
	Database              string        `json:"database" mapstructure:"database"`           // SQLite 文件名或路径
	MaxIdleConnections    int           `json:"max-idle-connections,omitempty" mapstructure:"max-idle-connections,omitempty"`
	MaxOpenConnections    int           `json:"max-open-connections,omitempty" mapstructure:"max-open-connections"`
	MaxConnectionLifeTime time.Duration `json:"max-connection-life-time,omitempty" mapstructure:"max-connection-life-time"`
	LogLevel              int           `json:"log-level" mapstructure:"log-level"`
}

// NewSQLiteOptions 创建一个使用默认值的 SQLiteOptions 实例。
func NewSQLiteOptions() *SQLiteOptions {
	return &SQLiteOptions{
		Addr:                  "./data/sqlite.db", // 可以是文件路径或 ":memory:"（内存数据库）
		Username:              "",                 // SQLite 不需要用户名/密码，仅为配置兼容性保留
		Password:              "",
		Database:              "", // 为空时使用 Addr
		MaxIdleConnections:    2,
		MaxOpenConnections:    5,
		MaxConnectionLifeTime: time.Duration(30) * time.Second,
		LogLevel:              1, // Silent
	}
}

// Validate 校验传递给 SQLiteOptions 的命令行标志。
func (o *SQLiteOptions) Validate() []error {
	errs := []error{}
	return errs
}

// AddFlags 将 SQLite 相关的配置标志注入到 API 服务器的指定 FlagSet 中。
func (o *SQLiteOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Addr, fullPrefix+".path", o.Addr,
		"SQLite 数据库文件路径（例如：./data/app.db 或 file::memory:?cache=shared）。")
	fs.StringVar(&o.Username, fullPrefix+".username", o.Username,
		"用户名（为配置一致性保留的字段）。")
	fs.StringVar(&o.Password, fullPrefix+".password", o.Password,
		"密码（为配置一致性保留的字段）。")
	fs.StringVar(&o.Database, fullPrefix+".database", o.Database,
		"SQLite 的数据库名称或覆盖路径。")
	fs.IntVar(&o.MaxIdleConnections, fullPrefix+".max-idle-connections", o.MaxIdleConnections,
		"SQLite 的最大空闲连接数。")
	fs.IntVar(&o.MaxOpenConnections, fullPrefix+".max-open-connections", o.MaxOpenConnections,
		"SQLite 的最大打开连接数。")
	fs.DurationVar(&o.MaxConnectionLifeTime, fullPrefix+".max-connection-life-time", o.MaxConnectionLifeTime,
		"SQLite 数据库连接的最大生命周期。")
	fs.IntVar(&o.LogLevel, fullPrefix+".log-mode", o.LogLevel,
		"指定 GORM 日志级别。")
}

// DSN 根据选项构建 SQLite 的 DSN。
func (o *SQLiteOptions) DSN() string {
	if o.Database != "" {
		return o.Database
	}
	if o.Addr != "" {
		return o.Addr
	}
	// 默认数据库文件名
	return "./data/sqlite.db"
}

// NewDB 创建一个 GORM SQLite 数据库连接。
func (o *SQLiteOptions) NewDB() (*gorm.DB, error) {
	opts := &db.SQLiteOptions{
		Addr:                  o.Addr,
		Username:              o.Username, // SQLite 不使用，但保留以保持接口兼容性
		Password:              o.Password,
		Database:              o.Database,
		MaxIdleConnections:    o.MaxIdleConnections,
		MaxOpenConnections:    o.MaxOpenConnections,
		MaxConnectionLifeTime: o.MaxConnectionLifeTime,
		Logger:                log.Default(),
	}

	return db.NewSQLite(opts)
}
