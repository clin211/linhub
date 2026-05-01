package db

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// 默认的 Redis 健康检查 ping 超时（避免应用启动时无限等待）
const defaultPingTimeout = 5 * time.Second

// RedisOptions 定义 Redis 数据库的配置选项。
type RedisOptions struct {
	Addr         string
	Username     string
	Password     string
	Database     int
	MaxRetries   int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
	PoolSize     int
}

// NewRedis 使用给定的配置选项创建一个新的 Redis 实例。
// 创建时会执行带超时的 Ping 检测，Redis 不可达时不会无限阻塞应用启动。
func NewRedis(opts *RedisOptions) (*redis.Client, error) {
	options := &redis.Options{
		Addr:         opts.Addr,
		Username:     opts.Username,
		Password:     opts.Password,
		DB:           opts.Database,
		MaxRetries:   opts.MaxRetries,
		MinIdleConns: opts.MinIdleConns,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		PoolTimeout:  opts.PoolTimeout,
		PoolSize:     opts.PoolSize,
	}

	rdb := redis.NewClient(options)

	pingTimeout := opts.DialTimeout
	if pingTimeout <= 0 {
		pingTimeout = defaultPingTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		_ = rdb.Close()
		return nil, err
	}

	return rdb, nil
}
