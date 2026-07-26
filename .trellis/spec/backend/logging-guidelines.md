# Logging Guidelines

> 基于 zap 的全局 logger、component 命名约定与敏感信息脱敏。

---

## 日志基础设施

实现在 `internal/pkg/logger/`，底层 `go.uber.org/zap`，文件轮转用 `lumberjack.v2`。

特点：

- **全局单例**：`logger.L()` 返回 `*zap.Logger`，`logger.S()` 返回 `*zap.SugaredLogger`。不需要逐层注入 logger。
- **动态调级**：`logger.SetLevel("debug")` 运行时生效（管理端可改），无需重启。
- **Ops sink**：`logger.SetSink()` 把日志同时写入数据库支撑的系统日志（`OpsSystemLogSink`）。
- **标准库桥接**：`slog_handler.go` / `stdlog_bridge.go` 把 `log` 与 `log/slog` 的输出接管进 zap。
- 配置来自 `config.LogConfig`（`level` / `format` / `service_name` / `env` / `caller` / `stacktrace_level` /
  `output.to_stdout` / `output.to_file` / `rotation` / `sampling`）。

---

## 两套写法：优先结构化

### 1. 结构化日志（新代码首选）

```go
logger.L().Info("account scheduled",
	zap.Int64("account_id", account.ID),
	zap.String("platform", account.Platform),
	zap.Int64("duration_ms", d.Milliseconds()),
)

// 带固定字段的子 logger
l := logger.With(zap.String("component", "service.gateway"))

// 从 context 取（若上游用 logger.IntoContext 注入过）
logger.FromContext(ctx).Warn("upstream degraded")
```

### 2. `logger.LegacyPrintf`（存量主流写法）

```go
logger.LegacyPrintf("service.antigravity_gateway", "%s status=success duration_ms=%d", prefix, ms)
```

它把 printf 风格消息包成 zap 事件，自动注入 `component` 字段，并**根据消息内容推断级别**
（含 `error`/`fail` → Error，含 `warn` → Warn，否则 Info），同时打上 `legacy_printf=true`。

当前代码库中 `LegacyPrintf` 约 890 处、结构化写法约 149 处。
**新代码请写结构化日志**；只有在既有文件里保持一致性时才继续用 `LegacyPrintf`。

**禁止直接用标准库 `log.Printf` / `fmt.Println` 输出运行时日志**——会绕过级别控制、component 归类与 Ops sink。

---

## component 命名约定

`component` 是日志检索与 Ops 系统日志归类的主维度，格式为 **`<层>.<领域>`**，全小写下划线：

```
service.gateway            service.antigravity_gateway    service.auth
service.pricing            service.billing_cache          service.usage_cleanup
repository.account         repository.claude_oauth
handler.admin.ops_ws       handler.admin.usage
setup
```

规则：

- 第一段用包所在层：`service` / `repository` / `handler` / `middleware` / `setup`。
- 第二段用领域名，与文件名对齐（`antigravity_gateway_*.go` → `service.antigravity_gateway`）。
- 管理端可用三段：`handler.admin.<子域>`。
- **同一文件内 component 必须一致**，不要一个文件用两个名字。

---

## 日志级别

| 级别 | 用于 |
|------|------|
| `Debug` | 详细排障信息（默认不开），如上游请求体摘要、调度中间态 |
| `Info` | 正常业务里程碑：账号调度结果、任务完成、配置刷新 |
| `Warn` | 可自愈的异常：缓存写失败、单次上游超时、降级回退 |
| `Error` | 需要人工关注：事务失败、配置非法、上游持续不可用 |
| `Fatal` | 仅限启动阶段不可恢复的错误 |

热路径（网关每请求）注意量级：**拒绝风暴时不要输出高基数日志或每请求写库**
（见 `deploy/EDGE_SECURITY.md` 的 DDoS boundary 一节）。
需要保留标准日志但不入 Ops 系统日志表时，加字段 `logger.OpsSystemLogSkipField`。

---

## 脱敏（强制）

`internal/util/logredact` 负责敏感信息擦除。默认敏感 key：

```
authorization_code, code, code_verifier, access_token, refresh_token,
id_token, client_secret, password
```

另有正则匹配的凭据模式：Google OAuth client secret（`GOCSPX-…`）、Google API Key（`AIza…`）。

API：

```go
logredact.RedactText(s string, extraKeys ...string) string
logredact.RedactJSON(raw []byte, extraKeys ...string) string
logredact.RedactMap(m map[string]any, extraKeys ...string) map[string]any
```

**必须脱敏的场景**：

- 打印上游请求/响应体、OAuth 回调参数、账号 credentials。
- 打印任何来自 `Authorization` 头或 config 的字符串。

`response.ErrorFrom` 在写 5xx 日志时已自动调用 `logredact.RedactText`，
但**你自己写的日志不会自动脱敏**。

---

## 绝对不能记录的内容

- 完整 API Key / API Key 明文（只能记前缀或哈希）。
- JWT、refresh token、OAuth code / verifier。
- 用户密码、bcrypt hash。
- 账号 credentials（`internal/service/account_credentials_redact.go`、
  `internal/handler/dto/credentials_redact.go` 已有脱敏实现，复用它们）。
- 完整请求体中的用户 prompt 内容（Prompt 审计走 `internal/securityaudit/` 的专用通路，有独立权限控制）。

---

## 常见错误

- **用 `log.Printf` 输出业务日志** —— 绕过级别控制与 component 归类。
- **component 拼写不统一** —— 检索时漏日志。
- **在 for 循环 / 每请求路径打 Info** —— 高 QPS 下淹没日志、拖慢磁盘。
- **打印上游响应体不脱敏** —— 泄漏 token。
- **`LegacyPrintf` 消息里没有 error 关键词但实际是错误** —— 级别推断成 Info，告警漏报；这种情况改用
  `logger.L().Error(...)`。
