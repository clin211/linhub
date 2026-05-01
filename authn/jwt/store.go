package jwt

import (
	"context"
	"time"
)

// Storer 是 token 存储接口。
type Storer interface {
	// 存储 token 数据并指定过期时间。
	Set(ctx context.Context, accessToken string, expiration time.Duration) error

	// 从存储中删除 token 数据。
	Delete(ctx context.Context, accessToken string) (bool, error)

	// 检查 token 是否存在。
	Check(ctx context.Context, accessToken string) (bool, error)

	// 关闭存储。
	Close() error
}
