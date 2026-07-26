# Backend Architecture

> 分层边界、依赖方向与 DI 装配方式。这是本仓库唯一被 lint 强制的架构规范。

---

## 分层与依赖方向

```
cmd/server (main + wire)
      │
      ▼
internal/server        路由装配、HTTP server、中间件链
      │
      ▼
internal/handler       HTTP 边界：参数解析 / 校验 / DTO 映射 / 写响应
      │
      ▼
internal/service       业务逻辑 + 所有对外依赖的 interface 定义（端口）
      ▲
      │  implements
internal/repository    Ent / Redis / 外部 HTTP 客户端的具体实现（适配器）
      │
      ▼
ent/ (generated)  ·  Redis  ·  上游 LLM API
```

关键点：**依赖倒置**。`repository` 依赖 `service`，而不是反过来。

---

## 依赖倒置：接口定义在 service，实现在 repository

这是本仓库最重要的结构性约定，新增数据访问能力时必须遵守。

**接口定义在 `internal/service/`**（消费方定义端口）：

- `internal/service/account_service.go:50` — `AccountRepository`
- `internal/service/api_key_service.go:56` — `APIKeyRepository`
- `internal/service/group_service.go:16` — `GroupRepository`
- `internal/service/proxy_service.go:17` — `ProxyRepository`
- `internal/service/channel_service.go:29` — `ChannelRepository`

**实现在 `internal/repository/`**，构造函数返回 service 侧的接口类型：

```go
// internal/repository/user_repo.go
func NewUserRepository(client *dbent.Client, sqlDB *sql.DB) service.UserRepository {
	return newUserRepositoryWithSQL(client, sqlDB)
}

// 额外实现的接口用编译期断言钉死
var _ service.RedeemUserAdjustmentRepository = (*userRepository)(nil)
```

新增一个仓储的完整步骤：

1. 在对应的 `internal/service/xxx_service.go` 里定义 `XxxRepository interface`（只声明业务真正需要的方法）。
2. 在 `internal/repository/xxx_repo.go` 实现，构造函数签名返回 `service.XxxRepository`。
3. 把构造函数注册进 `internal/repository/wire.go` 的 `ProviderSet`。
4. 在 service 结构体里注入该 interface，并补齐单测 stub。

**反模式**：把 interface 定义在 `repository` 包再让 service 反向 import——会立刻被 depguard 拦截。
（`internal/repository/user_platform_quota_repo.go:52` 有一个 `UserPlatformQuotaRepository` 定义在 repository 侧，
它通过 `NewUserPlatformQuotaServiceAdapter` 适配到 service 端口，属于历史适配写法，不要作为新代码模板。）

---

## depguard 硬边界（CI 强制）

来自 `backend/.golangci.yml`：

| 规则 | 作用范围 | 禁止 import |
|------|----------|-------------|
| `service-no-repository` | `internal/service/**` | `internal/repository`、`gorm.io/gorm`、`redis/go-redis/v9` |
| `handler-no-repository` | `internal/handler/**` | `internal/repository`、`gorm.io/gorm`、`redis/go-redis/v9` |

`service` 侧有 6 个白名单豁免文件（`ops_aggregation_service.go`、`ops_alert_evaluator_service.go`、
`ops_cleanup_service.go`、`ops_metrics_collector.go`、`ops_scheduled_report_service.go`、`wire.go`）。
**不要往白名单里加新文件**——需要 Redis 就在 service 定义一个 cache interface，
在 repository 实现（参考 `internal/repository/gateway_cache.go`、`billing_cache.go`、`api_key_cache.go`）。

本地自查：

```bash
cd backend && golangci-lint run ./...
```

---

## Wire 依赖注入

编译期 DI，入口 `backend/cmd/server/wire.go`（`//go:build wireinject`），生成物 `wire_gen.go` 必须提交。

装配顺序（`initializeApplication`）：

```
config.ProviderSet
  → repository.ProviderSet
  → service.ProviderSet
  → securityaudit.ProviderSet
  → payment.ProviderSet
  → middleware.ProviderSet
  → handler.ProviderSet
  → server.ProviderSet
```

各包的 ProviderSet 位置：

| 包 | 文件 |
|----|------|
| config | `internal/config/wire.go:6` |
| repository | `internal/repository/wire.go:67` |
| service | `internal/service/wire.go:676` |
| payment | `internal/payment/wire.go:60` |
| middleware | `internal/server/middleware/wire.go:18` |
| handler | `internal/handler/wire.go:213` |
| server | `internal/server/http.go:24` |
| securityaudit | `internal/securityaudit/prompt_module.go:5` |

**新增组件时**：写好构造函数 → 加进对应包的 `ProviderSet` → 重新生成。

```bash
cd backend && go generate ./cmd/server   # 重新生成 wire_gen.go
```

需要读配置做条件构造时，写 `ProvideXxx(cfg *config.Config) service.Xxx` 形式的 provider
（示例：`internal/repository/wire.go` 的 `ProvideConcurrencyCache` / `ProvideSessionLimitCache` / `ProvideSchedulerCache`），
不要在构造函数内部去读全局配置。

### 长生命周期组件必须可停止

任何带后台 goroutine / ticker / 连接池的 service，都要提供 `Stop()`（或 `Shutdown(ctx)`），
并注册到 `cmd/server/wire.go` 的 `provideCleanup` 中。
该函数按「应用层并行停止 → 基础设施（Redis、Ent）顺序关闭」执行，整体 10s 超时。
**新增后台服务却不注册 cleanup 会导致进程无法优雅退出。**

---

## Handler 层职责

`internal/handler/` 只做 HTTP 边界工作，业务逻辑一律下沉 service。

标准形态（参考 `internal/handler/api_key_handler.go`）：

```go
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService   // 只依赖 service
}

func NewAPIKeyHandler(apiKeyService *service.APIKeyService) *APIKeyHandler { ... }

// Request DTO 与 handler 同文件，用 gin binding tag 做基础校验
type CreateAPIKeyRequest struct {
	Name   string `json:"name" binding:"required"`
	Status string `json:"status" binding:"omitempty,oneof=active inactive"`
}

// GET /api/v1/api-keys
func (h *APIKeyHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	...
}
```

约定：

- 每个 handler 方法上方注释写明 HTTP 方法与路径（`// GET /api/v1/api-keys`）。
- 身份统一通过 `middleware.GetAuthSubjectFromContext(c)` 获取，不要自己解析 token。
- 分页统一用 `response.ParsePagination(c)`，不要各自解析 `page`/`limit`。
- 出参映射走 `internal/handler/dto`（`mappers.go`、`credentials_redact.go`），敏感字段必须脱敏。
- 所有 handler 汇总到 `internal/handler/handler.go` 的 `Handlers` 结构体，由 wire 装配。
- 管理端 handler 放 `internal/handler/admin/`。

---

## 路由与中间件

路由装配在 `internal/server/router.go`（`SetupRouter` → `registerRoutes`），
按域拆分到 `internal/server/routes/`：`auth.go`、`user.go`、`admin.go`、`payment.go`、`gateway.go`、`common.go`。

全局中间件链（`SetupRouter` 内，顺序有语义，不要随意调整）：

```
RequestLogger → SessionBindingContext(cfg) → Logger → CORS → SecurityHeaders → ServerTiming
→ (embedded frontend middleware, 仅 -tags embed 构建)
```

两套 API 前缀，语义完全不同：

| 前缀 | 用途 | 认证 |
|------|------|------|
| `/api/v1/**` | 管理/用户控制台 API，返回项目自有信封 | JWT（`middleware.JWTAuthMiddleware`）；`/api/v1/admin/**` 再叠加 `AdminAuthMiddleware` |
| `/v1/**`、`/v1beta/**` | LLM 网关，兼容 Anthropic / OpenAI / Gemini 协议 | API Key（`middleware.APIKeyAuthMiddleware`） |
| `/health`、`/setup/**` | 健康检查与初始化引导 | 无 |

**网关路由的错误响应必须符合上游协议格式**，不能套用 `/api/v1` 的信封。
`internal/server/routes/gateway.go` 里按协议选择 error writer
（`middleware.AnthropicErrorWriter` / `middleware.GoogleErrorWriter`）就是这个原因。

---

## 平台抽象

多 LLM 平台通过 `internal/domain/constants.go` 的常量区分：

```go
PlatformAnthropic / PlatformOpenAI / PlatformGemini / PlatformAntigravity / PlatformGrok / PlatformComposite
```

协议适配代码放 `internal/pkg/<platform>/`：`claude/`、`openai/`、`openai_compat/`、`gemini/`、`geminicli/`、
`antigravity/`、`xai/`、`googleapi/`、`anthropicfp/`、`apicompat/`。

**新增平台时**：先加 `domain` 常量 → 在 `internal/pkg/` 建协议包 → 在 `internal/service/` 加 gateway service
→ 在 `routes/gateway.go` 接线。不要把协议细节散落进 handler。
