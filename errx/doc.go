// Package errx 提供基于业务错误码的统一错误处理。
//
// # 设计哲学
//
// 在前后端分离的微服务架构中，HTTP 状态码语义和业务错误码常常存在职责重叠
// 与歧义。本包采取"业务错误码优先"的设计：只要请求能成功到达业务层并返回
// 结构化响应，HTTP 状态码统一为 200，业务结果通过响应 body 中的 code 字段
// 区分；只有当系统级故障无法工作时才返回非 200 状态码。
//
// # 错误码格式
//
// 错误码遵循 LMMNN 五位数格式：
//
//   - L  错误级别（1=系统级、2=用户级、3=业务级、4=上游、5=下游）
//   - MM 业务模块（00=通用、01=用户、02=文章、03=评论、04=认证、05=数据库、06=缓存）
//   - NN 具体错误编号
//
// 示例：CodeUserNotFound = 20101 表示
//
//   - L=2 用户级
//   - MM=01 用户模块
//   - NN=01 用户不存在
//
// # 推荐用法
//
//   - 业务层：errx.NewBizError(code, reason, message)
//   - HTTP 响应：errx.WriteResponse(c, data, err)（详见 core 包）
//   - 错误判断：errx.Is(err, errx.ErrNotFound)
//   - 跨服务传递：通过 gRPC status 自动序列化（GRPCStatus 方法）
package errx
