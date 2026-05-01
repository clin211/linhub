package options

import (
	"time"

	"github.com/spf13/pflag"
	"gorm.io/gorm"

	"github.com/clin211/linhub/db"
	"github.com/clin211/linhub/log"
)

var _ IOptions = (*PostgreSQLOptions)(nil)

// PostgreSQLOptions 定义 PostgreSQL 数据库的选项。
type PostgreSQLOptions struct {
	Addr                  string        `json:"addr,omitempty" mapstructure:"addr"`
	Username              string        `json:"username,omitempty" mapstructure:"username"`
	Password              string        `json:"-" mapstructure:"password"`
	Database              string        `json:"database" mapstructure:"database"`
	MaxIdleConnections    int           `json:"max-idle-connections,omitempty" mapstructure:"max-idle-connections,omitempty"`
	MaxOpenConnections    int           `json:"max-open-connections,omitempty" mapstructure:"max-open-connections"`
	MaxConnectionLifeTime time.Duration `json:"max-connection-life-time,omitempty" mapstructure:"max-connection-life-time"`
	LogLevel              int           `json:"log-level" mapstructure:"log-level"`
}

// NewPostgreSQLOptions 创建一个 `zero` 值实例。
func NewPostgreSQLOptions() *PostgreSQLOptions {
	return &PostgreSQLOptions{
		Addr:                  "127.0.0.1:5432",
		Username:              "onex",
		Password:              "onex(#)666",
		Database:              "onex",
		MaxIdleConnections:    100,
		MaxOpenConnections:    100,
		MaxConnectionLifeTime: time.Duration(10) * time.Second,
		LogLevel:              1, // Silent
	}
}

// Validate 校验传递给 PostgreSQLOptions 的命令行标志。
func (o *PostgreSQLOptions) Validate() []error {
	errs := []error{}

	return errs
}

// AddFlags 将与特定 APIServer 的 PostgreSQL 存储相关的命令行标志添加到指定的 FlagSet。
func (o *PostgreSQLOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Addr, fullPrefix+".addr", o.Addr, ""+
		"PostgreSQL 服务地址。如果留空，下面相关的 PostgreSQL 选项将被忽略。")
	fs.StringVar(&o.Username, fullPrefix+".username", o.Username, "访问 PostgreSQL 服务的用户名。")
	fs.StringVar(&o.Password, fullPrefix+".password", o.Password, ""+
		"访问 PostgreSQL 的密码，应与用户名配对使用。")
	fs.StringVar(&o.Database, fullPrefix+".database", o.Database, ""+
		"服务器要使用的数据库名称。")
	fs.IntVar(&o.MaxIdleConnections, fullPrefix+".max-idle-connections", o.MaxOpenConnections, ""+
		"允许连接到 PostgreSQL 的最大空闲连接数。")
	fs.IntVar(&o.MaxOpenConnections, fullPrefix+".max-open-connections", o.MaxOpenConnections, ""+
		"允许连接到 PostgreSQL 的最大打开连接数。")
	fs.DurationVar(&o.MaxConnectionLifeTime, fullPrefix+".max-connection-life-time", o.MaxConnectionLifeTime, ""+
		"允许连接到 PostgreSQL 的最大连接生命周期。")
	fs.IntVar(&o.LogLevel, fullPrefix+".log-mode", o.LogLevel, ""+
		"指定 gorm 日志级别。")
}

// NewDB 使用给定的配置创建 PostgreSQL 存储。
func (o *PostgreSQLOptions) NewDB() (*gorm.DB, error) {
	opts := &db.PostgreSQLOptions{
		Addr:                  o.Addr,
		Username:              o.Username,
		Password:              o.Password,
		Database:              o.Database,
		MaxIdleConnections:    o.MaxIdleConnections,
		MaxOpenConnections:    o.MaxOpenConnections,
		MaxConnectionLifeTime: o.MaxConnectionLifeTime,
		Logger:                log.Default(),
	}

	return db.NewPostgreSQL(opts)
}
