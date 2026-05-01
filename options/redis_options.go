package options

import (
	"time"

	"github.com/redis/go-redis/extra/rediscensus/v9"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/pflag"

	"github.com/clin211/linhub/db"
)

var _ IOptions = (*RedisOptions)(nil)

// RedisOptions 定义 Redis 集群的选项。
type RedisOptions struct {
	Addr         string        `json:"addr" mapstructure:"addr"`
	Username     string        `json:"username" mapstructure:"username"`
	Password     string        `json:"password" mapstructure:"password"`
	Database     int           `json:"database" mapstructure:"database"`
	MaxRetries   int           `json:"max-retries" mapstructure:"max-retries"`
	MinIdleConns int           `json:"min-idle-conns" mapstructure:"min-idle-conns"`
	DialTimeout  time.Duration `json:"dial-timeout" mapstructure:"dial-timeout"`
	ReadTimeout  time.Duration `json:"read-timeout" mapstructure:"read-timeout"`
	WriteTimeout time.Duration `json:"write-timeout" mapstructure:"write-timeout"`
	PoolTimeout  time.Duration `json:"pool-time" mapstructure:"pool-time"`
	PoolSize     int           `json:"pool-size" mapstructure:"pool-size"`
	// 链路追踪开关
	EnableTrace bool `json:"enable-trace" mapstructure:"enable-trace"`
}

// NewRedisOptions 创建一个 `zero` 值实例。
func NewRedisOptions() *RedisOptions {
	return &RedisOptions{
		Addr:         "127.0.0.1:6379",
		Username:     "",
		Password:     "",
		Database:     0,
		MaxRetries:   3,
		MinIdleConns: 0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		EnableTrace:  false,
	}
}

// Validate 校验传递给 RedisOptions 的命令行标志。
func (o *RedisOptions) Validate() []error {
	errs := []error{}

	if o.WriteTimeout == 0 {
		o.WriteTimeout = o.ReadTimeout
	}

	if o.PoolTimeout == 0 {
		o.PoolTimeout = o.ReadTimeout + 1*time.Second
	}

	return errs
}

// AddFlags 将与特定 APIServer 的 Redis 存储相关的命令行标志添加到指定的 FlagSet。
func (o *RedisOptions) AddFlags(fs *pflag.FlagSet, fullPrefix string) {
	fs.StringVar(&o.Addr, fullPrefix+".addr", o.Addr, "Redis 服务器地址（ip:port）。")
	fs.StringVar(&o.Username, fullPrefix+".username", o.Username, "访问 Redis 服务的用户名。")
	fs.StringVar(&o.Password, fullPrefix+".password", o.Password, "Redis 数据库的可选认证密码。")
	fs.IntVar(&o.Database, fullPrefix+".database", o.Database, "连接到服务器后要选择的数据库。")
	fs.IntVar(&o.MaxRetries, fullPrefix+".max-retries", o.MaxRetries, "放弃前的最大重试次数。")
	fs.IntVar(&o.MinIdleConns, fullPrefix+".min-idle-conns", o.MinIdleConns, ""+
		"最小空闲连接数，在建立新连接较慢时非常有用。")
	fs.DurationVar(&o.DialTimeout, fullPrefix+".dial-timeout", o.DialTimeout, "建立新连接的拨号超时时间。")
	fs.DurationVar(&o.ReadTimeout, fullPrefix+".read-timeout", o.ReadTimeout, "套接字读取超时时间。")
	fs.DurationVar(&o.WriteTimeout, fullPrefix+".write-timeout", o.WriteTimeout, "套接字写入超时时间。")
	fs.DurationVar(&o.PoolTimeout, fullPrefix+".pool-timeout", o.PoolTimeout, ""+
		"当所有连接都在忙时，客户端在返回错误之前等待连接的时间。")
	fs.IntVar(&o.PoolSize, fullPrefix+".pool-size", o.PoolSize, "套接字连接的最大数量。")
	fs.BoolVar(&o.EnableTrace, fullPrefix+".enable-trace", o.EnableTrace, "Redis hook 链路追踪（使用 OpenTelemetry）。")
}

func (o *RedisOptions) NewClient() (*redis.Client, error) {
	opts := &db.RedisOptions{
		Addr:         o.Addr,
		Username:     o.Username,
		Password:     o.Password,
		Database:     o.Database,
		MaxRetries:   o.MaxRetries,
		MinIdleConns: o.MinIdleConns,
		DialTimeout:  o.DialTimeout,
		ReadTimeout:  o.ReadTimeout,
		WriteTimeout: o.WriteTimeout,
		PoolSize:     o.PoolSize,
		PoolTimeout:  o.PoolTimeout,
	}

	rdb, err := db.NewRedis(opts)
	if err != nil {
		return nil, err
	}

	// hook 链路追踪（使用 OpenTelemetry）
	if o.EnableTrace {
		rdb.AddHook(rediscensus.NewTracingHook())
	}

	return rdb, nil
}
