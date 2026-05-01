package authz

import (
	"fmt"
	"time"

	casbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	adapter "github.com/casbin/gorm-adapter/v3"
	"github.com/google/wire"
	"gorm.io/gorm"
)

const (
	// 默认的 Casbin 访问控制模型.
	defaultAclModel = `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft

[role_definition]
g = _, _

[policy_effect]
e = !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.sub) && keyMatch(r.obj, p.obj) && r.act == p.act`
)

// Authz 定义了一个授权器，提供授权功能.
type Authz struct {
	*casbin.SyncedEnforcer // 使用 Casbin 的同步授权器
}

// Option 定义了一个函数选项类型，用于自定义 NewAuthz 的行为.
type Option func(*authzConfig)

// authzConfig 是授权器的配置结构.
type authzConfig struct {
	aclModel           string        // Casbin 的模型字符串
	autoLoadPolicyTime time.Duration // 自动加载策略的时间间隔
}

// ProviderSet 是一个 Wire 的 Provider 集合，用于声明依赖注入的规则。
// 包含 NewAuthz 构造函数，用于生成 Authz 实例。
var ProviderSet = wire.NewSet(NewAuthz, DefaultOptions)

// defaultAuthzConfig 返回一个默认的配置.
func defaultAuthzConfig() *authzConfig {
	return &authzConfig{
		// 默认使用内置的 ACL 模型
		aclModel: defaultAclModel,
		// 默认的自动加载策略时间间隔
		autoLoadPolicyTime: 5 * time.Second,
	}
}

// DefaultOptions 提供默认的授权器选项配置.
func DefaultOptions() []Option {
	return []Option{
		// 使用默认的 ACL 模型
		WithAclModel(defaultAclModel),
		// 设置自动加载策略的时间间隔为 10 秒
		WithAutoLoadPolicyTime(10 * time.Second),
	}
}

// WithAclModel 允许通过选项自定义 ACL 模型.
func WithAclModel(model string) Option {
	return func(cfg *authzConfig) {
		cfg.aclModel = model
	}
}

// WithAutoLoadPolicyTime 允许通过选项自定义自动加载策略的时间间隔.
func WithAutoLoadPolicyTime(interval time.Duration) Option {
	return func(cfg *authzConfig) {
		cfg.autoLoadPolicyTime = interval
	}
}

// NewAuthz 创建一个使用 Casbin 完成授权的授权器，通过函数选项模式支持自定义配置.
func NewAuthz(db *gorm.DB, opts ...Option) (*Authz, error) {
	cfg := defaultAuthzConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	adapter, err := adapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin adapter: %w", err)
	}

	m, err := model.NewModelFromString(cfg.aclModel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse casbin acl model: %w", err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load casbin policy: %w", err)
	}

	enforcer.StartAutoLoadPolicy(cfg.autoLoadPolicyTime)

	return &Authz{SyncedEnforcer: enforcer}, nil
}

// Authorize 用于进行授权.
func (a *Authz) Authorize(sub, obj, act string) (bool, error) {
	return a.Enforce(sub, obj, act)
}

// Stop 停止 Casbin 的后台 AutoLoadPolicy goroutine，避免泄露。
// 在重新创建 Authz 实例或关停应用时应调用此方法。
func (a *Authz) Stop() {
	if a == nil || a.SyncedEnforcer == nil {
		return
	}
	a.StopAutoLoadPolicy()
}
