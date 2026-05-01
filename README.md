# linhub

`linhub` 是面向 Go 后端的一组**可复用基础库**（Go module：`github.com/clin211/linhub`）。它提供统一的业务错误模型、结构化日志、数据库与缓存接入、HTTP（Gin）与 gRPC 侧常用模式、认证与鉴权、可配置选项（options）、OpenTelemetry 等能力，目的是让多个服务在**错误语义、日志字段、存储访问与观测性**上保持一致，而不是替代 Gin、GORM 或 gRPC 等基础框架。

**适用**：中后台 API、微服务、需要统一 JSON 业务码与观测性的项目。  
**不适用**：希望完全零依赖、或强绑定另一套全局错误/日志规范且无迁移成本的项目。

---

## 环境要求

| 项目 | 说明 |
| --- | --- |
| Go | **1.25.3**（以仓库根 `go.mod` 的 `go` 行为准；升级时请同步修改本表） |
| 模块 | `github.com/clin211/linhub` |

请尽量使用与仓库一致的 Go 版本，减少 `toolchain` 与校验和差异。

---

## 安装与版本锁定

```bash
go get github.com/clin211/linhub@vX.Y.Z
# 或固定到 commit
go get github.com/clin211/linhub@abcdef1
```

生产环境建议**固定 tag 或 commit**，并将 `go.sum` 纳入审计。

本地 monorepo 可将模块指到同级目录：

```go
replace github.com/clin211/linhub => ../linhub
```

---

## 仓库布局（顶层）

```
linhub/
├── api/              # 公共 API / 生成物相关（视项目而定）
├── app/              # 应用级组合与生命周期
├── authn/            # 认证（含 jwt 等子包）
├── authz/            # 鉴权（Casbin + GORM adapter）
├── binding/          # HTTP 绑定辅助
├── core/             # Gin 请求处理、与 errx 配合的响应写法
├── db/               # SQL / Redis / Mongo 等连接工厂
├── errx/             # 业务错误码与 gRPC/HTTP 约定
├── i18n/             # 国际化
├── id/               # ID 相关
├── log/              # 基于 slog 的结构化日志
├── logger/           # 日志侧扩展
├── middleware/       # gin / grpc 中间件
├── options/          # 各组件 Options + pflag
├── otel/             # OpenTelemetry 初始化
├── otelslog/         # 日志与 OTel 桥接
├── ptr/              # 指针辅助
├── rid/              # 请求/关联 ID 等
├── server/           # HTTP、gRPC 服务启动辅助
├── store/            # GORM 泛型 Store、where、registry
├── token/            # Token 相关
├── util/             # 分页、重试、文件、字符串等工具子树
├── validation/       # 校验扩展
└── version/          # 版本元信息
```

更细的说明见下表与各包 `doc.go`。

---

## 包结构一览

| 包路径 | 职责摘要 |
| --- | --- |
| `errx` | 业务错误 `BizCode`（LMMNN）、与 gRPC `status` 互通、标准响应头常量（如 `X-Request-ID`） |
| `core` | Gin：绑定 → 校验 → 业务函数 → 统一 JSON 响应（与 `errx` 一致） |
| `binding` | 供 `core` 使用的请求绑定 |
| `log` | 基于 `log/slog` 的 `Logger`；`Infow`、`W(ctx)` 等便捷方法；可实现 `gorm.io/gorm/logger.Interface` |
| `logger` | 与 `log` 配合的扩展能力 |
| `db` | PostgreSQL、MySQL、SQLite、Redis、Mongo 等连接的选项与构造函数 |
| `store` | 泛型 GORM 仓储、`store/where` 条件、`store/registry` 等 |
| `options` | PostgreSQL/MySQL/SQLite/Mongo/Redis/gRPC/HTTP/Kafka/OTel… 等 `*Options` 与 `pflag` |
| `authn` | 认证（如 `authn/jwt`） |
| `authz` | Casbin 同步 `SyncedEnforcer` + GORM 适配器封装 |
| `token` | Token 签发、校验、轮换等 |
| `middleware/gin` | 可观测性、trace 注入、访问日志等 |
| `middleware/grpc` | gRPC 侧通用中间件模式 |
| `server` | HTTP/gRPC 服务装配、健康检查与指标等 |
| `app` | 应用入口组合 |
| `otel` / `otelslog` | 链路、指标、日志导出与初始化 |
| `i18n` | go-i18n 集成 |
| `id` / `rid` | ID / 请求 ID 生成与传递 |
| `validation` | 校验相关扩展 |
| `version` | 二进制版本信息 |
| `util/...` | 分页、重试、反射、文件、字符串、IP、lint 辅助等 |
| `api` | 与 IDL/公共契约相关的目录（若有） |

---

## 设计理念

### 错误与 HTTP：`errx`

- 业务层**可预期**的失败：优先 **HTTP 200**，在 body 中用 **`code`** 表示业务结果（详见 `errx/doc.go`）。
- **系统级**不可用：使用 **5xx**（或网关类错误码），与业务码区分。
- 错误码 **LMMNN**：级别、模块、序号分段，便于监控归类与跨服务对齐。

Handler 建议使用 `core.HandleJSONRequest` / `HandleUriRequest` / `HandleQueryRequest` 等，保持响应结构一致。

### 日志：`log`

- 统一走 `github.com/clin211/linhub/log`，在标准 `slog` 之上提供团队一致的 API（含 `W(ctx)` 从上下文抽取字段）。
- 可与 GORM 日志接口对接，减少重复配置。

### 数据访问：`db` + `store`

- `db`：连接串默认值、超时、池化等开箱实践。
- `store`：在 `*gorm.DB` 上提供泛型 CRUD、`Where`、事务与 `context` 传递。

### 配置：`options`

- 典型模式：`NewXxxOptions()` + `AddFlags` + `Validate` + `NewDB()` / `NewClient()`，既可接 `cobra`/`pflag`，也可被 `viper.Unmarshal` 填充。

### 可观测性

- `otel`、`middleware`、`otelslog` 协同：链路（trace）、指标（metrics）、日志 Correlation。按环境启用导出器（OTLP、Prometheus、stdout 等），见各 `options` 子包。

### 安全与合规

- 密钥与连接串勿写死于仓库；生产请使用环境变量或密钥托管。
- JWT、Casbin 策略、数据库账号应遵循最小权限；定期轮换。
- 依赖中含 `golang.org/x/crypto`、`jwt` 等，请跟踪上游安全通告。

---

## 快速开始

### 日志

```go
import "github.com/clin211/linhub/log"

func main() {
    log.Infow("listen", "addr", ":8080")
}
```

### PostgreSQL + GORM

```go
import (
    "github.com/clin211/linhub/db"
    "github.com/clin211/linhub/log"
)

gdb, err := db.NewPostgreSQL(&db.PostgreSQLOptions{
    Addr:     "127.0.0.1:5432",
    Username: "postgres",
    Password: "postgres",
    Database: "app",
    Logger:   log.Default(),
})
```

### Gin 与统一响应

```go
import (
    "context"
    "github.com/clin211/linhub/core"
    "github.com/gin-gonic/gin"
)

func route(r gin.IRoutes) {
    r.POST("/v1/items", func(c *gin.Context) {
        core.HandleJSONRequest(c, func(ctx context.Context, req *CreateReq) (*CreateResp, error) {
            return &CreateResp{}, nil
        })
    })
}
```

### MongoDB（Options）

```go
import "github.com/clin211/linhub/options"

o := options.NewMongoOptions()
o.URL = "mongodb://127.0.0.1:27017"
o.Database = "app"
o.Collection = "items"
client, err := o.NewClient()
```

---

## 测试

```bash
go test ./...
```

部分测试可能依赖外部进程或网络；失败时请阅读对应 `_test.go` 顶部说明。

---

## 工具链集成（可选）

命令行脚手架 [linctl](https://github.com/clin211/lin) 生成的项目常默认依赖本模块，并可在 monorepo 中使用 `replace` 指向本地 `linhub`。业务代码应**直接 import 本模块**，避免在业务仓库中复制与 `linhub` 重复的底座代码。

---

## 版本与兼容性

- 以 **tag / commit** 为版本边界；升级前阅读 `go.mod` 主要依赖（Gin、gorm、grpc、otel 等）是否引入破坏性变更。
- 导出 API 以 Go 文档与源码为准；跨大版本升级建议在预发环境做回归（尤其 `errx` 码表与 JSON契约）。

---

## 文档索引

- 各包 **`doc.go`**：设计说明与推荐用法（如 `errx/doc.go`）。
- 发布至公共代理后：可在 [pkg.go.dev](https://pkg.go.dev/github.com/clin211/linhub) 浏览（需模块可被代理抓取）。

---

## 常见问题

**1. 是否必须 HTTP 200 + body code？**  
`errx` 与 `core` 的默认组合按此约定实现；若你必须严格使用 REST 状态码映射，需在网关或 handler 层自行封装，与本库默认行为可能不一致。

**2. 依赖体积较大？**  
本模块聚合了存储、RPC、OTel、K8s client 等可选能力；未使用的包不会进入最终二进制，但 `go mod` 解析仍会拉取依赖树，可按需在业务侧用拆分模块或 build tag 进一步瘦身（需自行评估）。

**3. `store` 是否支持非 GORM？**  
当前 `store` 包以 GORM 为中心；其他持久化可仅用 `db` 中的工厂或直接用官方 driver。
