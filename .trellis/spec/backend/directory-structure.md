# Backend Directory Structure

> `backend/` 的实际布局与「新代码该放哪」的判定规则。

---

## 顶层布局

```
backend/
├── cmd/
│   ├── server/                    # 主程序入口 + wire DI（main.go / wire.go / wire_gen.go / VERSION）
│   ├── jwtgen/                    # JWT 生成小工具
│   └── cleanup-ingress-reject-logs/
├── ent/                           # Ent ORM 生成代码（约 13 万行，必须提交）
│   ├── schema/                    # ★ 手写的实体定义（唯一可改的地方）
│   │   └── mixins/                # TimeMixin / SoftDeleteMixin
│   ├── migrate/ hook/ intercept/ predicate/ runtime/ enttest/
│   └── <entity>/                  # 每个实体的生成包
├── internal/
│   ├── config/                    # viper 配置加载、校验、env 覆盖
│   ├── domain/                    # 跨层共享的常量与纯领域类型（无依赖）
│   ├── model/                     # 少量独立数据模型
│   ├── handler/                   # HTTP 边界
│   │   ├── admin/                 # 管理端 handler
│   │   ├── dto/                   # 出参映射与脱敏
│   │   └── quotaview/
│   ├── service/                   # 业务逻辑 + 端口 interface（最大的包，~944 个文件）
│   │   ├── openai_ws_v2/  prompts/  testdata/
│   ├── repository/                # Ent / Redis / 外部 HTTP 的实现（~269 个文件）
│   ├── server/                    # HTTP server、路由装配
│   │   ├── routes/                # 按域拆分的路由注册
│   │   └── middleware/            # 认证、CORS、安全头、审计、限流拒绝等
│   ├── middleware/                # 独立限流器（rate_limiter.go）
│   ├── pkg/                       # 可复用基础设施（见下）
│   ├── payment/  provider/        # 支付（Stripe / 支付宝 / 微信 / Airwallex）
│   ├── securityaudit/             # Prompt 审计子系统（自带 ProviderSet）
│   ├── platform/liveattestation/
│   ├── util/                      # httputil / logredact / responseheaders / urlvalidator
│   ├── setup/                     # 首次安装引导
│   ├── testutil/                  # 测试夹具与 stub（build tag: unit）
│   ├── integration/               # e2e 测试
│   └── web/                       # 前端静态资源内嵌（build tag: embed）
├── migrations/                    # 手写 SQL 迁移，NNN_描述.sql，共 242 个
├── resources/model-pricing/       # 内置模型定价数据
├── scripts/                       # resolve-version.sh / e2e-test.sh 等
├── .golangci.yml                  # ★ 架构边界与 lint 规则
├── Makefile
└── go.mod
```

---

## `internal/pkg/` 的职责划分

`pkg` 下按「协议适配」与「通用基础设施」两类组织：

**协议适配**（每个上游 LLM 一个包）：
`claude/`、`openai/`、`openai_compat/`、`gemini/`、`geminicli/`、`antigravity/`、`xai/`、
`googleapi/`、`anthropicfp/`、`apicompat/`、`websearch/`

**通用基础设施**：

| 包 | 职责 |
|----|------|
| `errors/` | `ApplicationError` 及 HTTP 映射 |
| `response/` | 统一响应信封与分页 |
| `logger/` | zap 全局 logger、slog 桥接、Ops sink |
| `httpclient/` `httputil/` | 上游 HTTP 客户端与请求工具 |
| `tlsfingerprint/` | uTLS 指纹伪装 |
| `proxyurl/` `proxyutil/` `ip/` | 代理与 IP 解析 |
| `pagination/` | 分页参数与结果 |
| `ctxkey/` | context key 统一定义 |
| `oauth/` | OAuth 通用流程 |
| `servertiming/` `usagestats/` `sysutil/` `timezone/` | 观测与系统工具 |

---

## 新代码落点判定

| 你要写的东西 | 放这里 |
|--------------|--------|
| 新的 HTTP 端点 | `internal/handler/`（管理端 → `handler/admin/`）+ `internal/server/routes/` 注册 |
| 新的业务规则 | `internal/service/` |
| 新的数据访问 / 缓存 / 外部 API 调用 | interface 定义在 `internal/service/`，实现在 `internal/repository/` |
| 新的数据库表 / 字段 | `ent/schema/*.go` + `migrations/NNN_*.sql` |
| 跨层共享常量（状态、角色、平台名） | `internal/domain/constants.go` |
| 新的上游协议适配 | `internal/pkg/<platform>/` |
| 新的 gin 中间件 | `internal/server/middleware/` |
| 出参脱敏 / DTO 映射 | `internal/handler/dto/` |

---

## 文件命名约定

- 全小写 + 下划线：`api_key_service.go`、`account_repo.go`、`gateway_handler_chat_completions.go`。
- 仓储文件统一 `*_repo.go`；缓存实现统一 `*_cache.go`（都在 `internal/repository/`）。
- Service 文件多为 `<domain>_service.go`，同一领域拆多文件时用 `<domain>_<子主题>.go`
  （例：`antigravity_gateway_service.go` / `_streaming.go` / `_retry.go` / `_upstream.go`）。
- 测试文件与被测文件同目录、同前缀：`account_service.go` ↔ `account_service_test.go`。
- 管理端 service 用 `admin_` 前缀：`admin_account.go`、`admin_group.go`、`admin_user.go`。

**大文件优先按子主题水平拆分**，而不是塞进一个文件——`internal/service/` 里 900+ 文件正是这个约定的结果。

---

## 禁止改动的目录

- `ent/` 除 `ent/schema/` 外全部为生成代码，手改会被 `go generate ./ent` 覆盖。
- `cmd/server/wire_gen.go` 由 `go generate ./cmd/server` 生成，只改 `wire.go`。
