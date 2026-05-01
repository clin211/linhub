package db

import (
	"github.com/google/wire"
	redis "github.com/redis/go-redis/v9"
)

// ProviderSet 是 db 的依赖提供者集合。
var ProviderSet = wire.NewSet(
	NewMySQL,
	NewRedis,
	wire.Bind(new(redis.UniversalClient), new(*redis.Client)), // 正确绑定接口和实现
)
