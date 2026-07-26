# Error Handling

> `ApplicationError` 类型、统一响应信封、以及网关协议错误的特殊处理。

---

## 核心类型：ApplicationError

定义在 `internal/pkg/errors/errors.go`。**`Code` 字段直接就是 HTTP 状态码**，不是自定义业务码。

```go
type Status struct {
	Code     int32             `json:"code"`
	Reason   string            `json:"reason,omitempty"`
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type ApplicationError struct {
	Status
	cause error   // 私有，通过 WithCause 设置，Unwrap 暴露
}
```

字段语义：

| 字段 | 用途 | 面向 |
|------|------|------|
| `Code` | HTTP 状态码（400/401/403/404/409/429/500） | 传输层 |
| `Reason` | 机器可读的稳定错误标识，英文 UPPER_SNAKE 风格 | 前端分支判断 |
| `Message` | 人类可读描述 | 展示 |
| `Metadata` | 结构化补充字段 | 前端渲染细节 |

---

## 构造错误

优先使用 `internal/pkg/errors/types.go` 的语义化构造器，不要手写 `New(400, ...)`：

```go
errors.BadRequest(reason, message)       // 400
errors.Unauthorized(reason, message)     // 401
errors.Forbidden(reason, message)        // 403
errors.NotFound(reason, message)         // 404
errors.TooManyRequests(reason, message)  // 429
```

附加上下文：

```go
// 保留底层 cause，供日志与 errors.Is/As 使用
return errors.BadRequest("INVALID_GROUP", "group not found").WithCause(err)

// 附加结构化元数据（会深拷贝）
return errors.TooManyRequests("QUOTA_EXCEEDED", "quota exceeded").
	WithMetadata(map[string]string{"reset_at": resetAt.Format(time.RFC3339)})
```

`WithCause` / `WithMetadata` **返回克隆**，不修改原对象——可以安全地把 `errors.BadRequest(...)`
的结果存为包级变量再逐次附加上下文。

---

## 判定错误

用 `types.go` 提供的判定函数，支持 wrapped error：

```go
if errors.IsNotFound(err)      { ... }
if errors.IsForbidden(err)     { ... }
if errors.IsTooManyRequests(err) { ... }

code   := errors.Code(err)     // 非 ApplicationError 时返回 500
reason := errors.Reason(err)
```

`errors.FromError(err)` 会把任意 error 归一化：不是 `ApplicationError` 的一律降级为
`500 / UnknownReason / "internal error"`，原始 error 作为 cause 保留。
**这意味着底层实现细节不会泄漏到响应体**，但也意味着未包装的错误一律呈现为 500。

---

## 统一响应信封

`internal/pkg/response/response.go` 定义了 `/api/v1/**` 的响应格式：

```go
type Response struct {
	Code     int               `json:"code"`
	Message  string            `json:"message"`
	Reason   string            `json:"reason,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Data     any               `json:"data,omitempty"`
}
```

**成功时 `code == 0`**（不是 200）；失败时 `code` 等于 HTTP 状态码。
前端 `frontend/src/api/client.ts` 的响应拦截器正是以 `code === 0` 判定成功并拆包 `data`，
改动信封必须同步前端。

Handler 侧写法：

```go
response.Success(c, data)                       // 200 + code:0
response.Created(c, data)                       // 201
response.Accepted(c, data)                      // 202（异步任务）
response.Paginated(c, items, total, page, size) // 分页
response.PaginatedWithResult(c, items, pg)

response.BadRequest(c, "message")               // 简单错误
response.ErrorFrom(c, err)                      // ★ 从 error 自动映射（推荐）
```

`response.ErrorFrom` 会：

1. 调 `errors.ToHTTP(err)` 拿到状态码与 Status 体；
2. 状态码 ≥ 500 时自动打印带 `logredact.RedactText` 脱敏的错误日志；
3. 写出信封并返回 `true`。

**推荐做法**：service 返回 `*ApplicationError`，handler 一律 `if response.ErrorFrom(c, err) { return }`，
不要在 handler 里逐个 `switch` 错误类型再手写状态码。

---

## 错误传播约定

| 层 | 职责 |
|----|------|
| `repository` | 返回原始错误或包装为 `ApplicationError`；不写日志、不决定 HTTP 语义 |
| `service` | 把领域失败转成语义化 `ApplicationError`（带 `Reason`）；用 `WithCause` 保留底层错误 |
| `handler` | 只做 `response.ErrorFrom(c, err)` 或显式 `response.XxxError(c, msg)` |
| `middleware` | 认证/限流失败直接写响应并 `c.Abort()` |

不要在 service 层写 `c *gin.Context`——service 不应感知 HTTP。

---

## 网关路由的错误格式不同

`/v1/**`、`/v1beta/**` 是 LLM 协议兼容层，**错误响应必须符合上游协议格式**，不能套用项目信封。

`internal/server/routes/gateway.go` 按协议选择 error writer：

```go
requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
requireGroupGoogle    := middleware.RequireGroupAssignment(settingService, middleware.GoogleErrorWriter)
```

新增网关端点时，**先确认客户端（Claude Code / Codex CLI / Gemini CLI）期望的错误结构**，
再选对应的 error writer。用错格式会导致客户端无法解析、重试逻辑失效。

相关实现参考：`internal/handler/concurrency_error_response.go`、
`internal/handler/gateway_handler_error_fallback_test.go`、`internal/service/error_passthrough_service.go`
（上游错误透传规则）。

---

## panic 处理

`internal/server/middleware/recovery.go` 负责兜底。业务代码里禁止用 panic 表达可预期的失败。

---

## 常见错误

- **手写 `errors.New(500, ...)` 而不用语义化构造器** —— 丢失 reason，前端无法分支。
- **在 handler 里 `switch err` 手写状态码** —— 用 `response.ErrorFrom`。
- **把上游 API 的原始错误直接透传到 `/api/v1`** —— 会泄漏 base URL / key 片段，先包装。
- **给网关端点返回项目信封** —— 客户端解析失败，见上一节。
- **`WithCause` 后忘了它返回克隆** —— `err.WithCause(x)` 不写回变量等于没生效。
