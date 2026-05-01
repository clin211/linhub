package options

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"gorm.io/gorm"

	"github.com/clin211/linhub/db"
	"github.com/clin211/linhub/log"
)

var _ IOptions = (*MySQLOptions)(nil)

// MySQLOptions 定义 MySQL 数据库的选项。
type MySQLOptions struct {
	Addr                  string        `json:"addr,omitempty" mapstructure:"addr"`
	Username              string        `json:"username,omitempty" mapstructure:"username"`
	Password              string        `json:"-" mapstructure:"password"`
	Database              string        `json:"database" mapstructure:"database"`
	MaxIdleConnections    int           `json:"max-idle-connections,omitempty" mapstructure:"max-idle-connections,omitempty"`
	MaxOpenConnections    int           `json:"max-open-connections,omitempty" mapstructure:"max-open-connections"`
	MaxConnectionLifeTime time.Duration `json:"max-connection-life-time,omitempty" mapstructure:"max-connection-life-time"`
	LogLevel              int           `json:"log-level" mapstructure:"log-level"`
}

// NewMySQLOptions 创建一个 `zero` 值实例。
// 默认连接池针对单个微服务实例的合理资源占用，详见 db.setMySQLDefaults。
func NewMySQLOptions() *MySQLOptions {
	return &MySQLOptions{
		Addr:                  "127.0.0.1:3306",
		Username:              "",
		Password:              "",
		Database:              "",
		MaxIdleConnections:    10,
		MaxOpenConnections:    20,
		MaxConnectionLifeTime: 30 * time.Minute,
		LogLevel:              1, // Silent
	}
}

// Validate 校验传递给 MySQLOptions 的命令行标志。
func (o *MySQLOptions) Validate() []error {
	errs := []error{}

	return errs
}

// AddFlags 将与特定 APIServer 的 MySQL 存储相关的命令行标志添加到指定的 FlagSet。
func (o *MySQLOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Addr, fullPrefix+".host", o.Addr, ""+
		"MySQL 服务的主机地址。")
	fs.StringVar(&o.Username, fullPrefix+".username", o.Username, "访问 MySQL 服务的用户名。")
	fs.StringVar(&o.Password, fullPrefix+".password", o.Password, ""+
		"访问 MySQL 的密码，应与用户名配对使用。")
	fs.StringVar(&o.Database, fullPrefix+".database", o.Database, ""+
		"服务器要使用的数据库名称。")
	fs.IntVar(&o.MaxIdleConnections, fullPrefix+".max-idle-connections", o.MaxOpenConnections, ""+
		"允许连接到 MySQL 的最大空闲连接数。")
	fs.IntVar(&o.MaxOpenConnections, fullPrefix+".max-open-connections", o.MaxOpenConnections, ""+
		"允许连接到 MySQL 的最大打开连接数。")
	fs.DurationVar(&o.MaxConnectionLifeTime, fullPrefix+".max-connection-life-time", o.MaxConnectionLifeTime, ""+
		"允许连接到 MySQL 的最大连接生命周期。")
	fs.IntVar(&o.LogLevel, fullPrefix+".log-mode", o.LogLevel, ""+
		"指定 gorm 日志级别。")
}

// DSN 返回从 MySQLOptions 构建的 DSN（与 db.MySQLOptions.DSN 保持一致）。
func (o *MySQLOptions) DSN() string {
	return fmt.Sprintf(`%s:%s@tcp(%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=%t&loc=%s&timeout=10s&readTimeout=30s&writeTimeout=30s`,
		o.Username,
		o.Password,
		o.Addr,
		o.Database,
		true,
		"Local")
}

// NewDB 使用给定的配置创建 MySQL 存储。
func (o *MySQLOptions) NewDB() (*gorm.DB, error) {
	opts := &db.MySQLOptions{
		Addr:                  o.Addr,
		Username:              o.Username,
		Password:              o.Password,
		Database:              o.Database,
		MaxIdleConnections:    o.MaxIdleConnections,
		MaxOpenConnections:    o.MaxOpenConnections,
		MaxConnectionLifeTime: o.MaxConnectionLifeTime,
		Logger:                log.Default(),
	}

	return db.NewMySQL(opts)
}
