# Testing Guidelines

> build tag 分层体系、共享 stub/fixture 与各类测试的写法。

---

## build tag 三层体系（核心约定）

**每个测试文件都必须带 build tag**，否则不会被任何一条测试命令执行。

| Tag | 命令 | 范围 | 外部依赖 |
|-----|------|------|----------|
| `//go:build unit` | `make test-unit` / `go test -tags=unit ./...` | 单元测试（约 338 个文件） | 无（用 stub / sqlmock / miniredis） |
| `//go:build integration` | `make test-integration` / `go test -tags=integration ./...` | 集成测试（约 68 个文件） | testcontainers 拉起真实 PostgreSQL / Redis |
| `//go:build e2e` | `make test-e2e-local` / `./scripts/e2e-test.sh` | 端到端（`internal/integration/`） | 完整运行中的服务 |

另有 `//go:build embed`（`internal/web/embed_test.go`），仅在带前端产物的构建中生效。

CI（`.github/workflows/backend-ci.yml`）跑 unit + integration，不跑 e2e。

**新写测试时第一行就是 build tag。** 忘写的表现是「测试文件存在但从来没跑过」。

---

## 单元测试

### 框架与风格

- 断言库：`github.com/stretchr/testify`（793 个测试文件使用），`require` 用于致命前置条件，`assert` 用于普通断言。
- 表驱动测试是主流（268 个文件），标准形态：

```go
tests := []struct {
	name    string
	input   string
	want    string
	wantErr bool
}{
	{name: "empty input", input: "", want: "", wantErr: true},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) { ... })
}
```

- 子测试名用下划线连接的描述式命名：`t.Run("returns_true_when_frontend_embedded", ...)`。

### 替身选择

| 需要 | 用什么 | 示例 |
|------|--------|------|
| service 的仓储/缓存依赖 | `internal/testutil` 的 Stub | `testutil.StubConcurrencyCache{}` |
| Redis | `github.com/alicebob/miniredis/v2`（10 个文件） | — |
| 原始 SQL 层 | `github.com/DATA-DOG/go-sqlmock`（23 个文件） | — |
| 真实 Ent client（内存 SQLite） | `ent/enttest`（14 个文件） | — |
| HTTP handler | `internal/testutil/httptest.go` + `httptest` | 已在 init 里设好 `gin.SetMode(gin.TestMode)` |

---

## `internal/testutil` 共享设施

**整个包都是 `//go:build unit`**，所以不会进生产构建，也**无法**被 integration/e2e 测试引用。

- `fixtures.go` —— 函数选项式夹具：

```go
u := testutil.NewTestUser(func(u *service.User) { u.Balance = 0 })
a := testutil.NewTestAccount(func(a *service.Account) { a.Platform = service.PlatformOpenAI })
```

- `stubs.go` —— 接口的零值空实现，带编译期断言：

```go
var _ service.ConcurrencyCache = StubConcurrencyCache{}
```

- `httptest.go` —— gin 测试上下文与请求构造工具。

**给 stub 加方法的时机**：只要给 `service` 里的 interface 新增方法，
`testutil/stubs.go` 里对应的 Stub 以及各测试文件里的临时 mock 都必须补齐，否则编译失败：

```
does not implement interface (missing method XXX)
```

定位方式：

```bash
cd backend
grep -r "type.*Stub.*struct" internal/
grep -r "type.*Mock.*struct" internal/
```

这是本项目的高频踩坑点（`DEV_GUIDE.md` 坑 6）。加方法前先评估能否用更窄的新接口代替扩展旧接口。

---

## 契约测试

`internal/server/api_contract_test.go`（`//go:build unit`）锁定 `/api/v1` 的响应契约：
信封字段、分页结构、状态码。

**任何改动响应格式的 PR 都要同步这个文件**，因为前端 `frontend/src/api/client.ts`
的响应拦截器直接依赖 `code === 0` + `data` 拆包。

相关：`internal/server/http_ingress_test.go`（ingress 限制）、
`internal/server/routes/*_test.go`（各路由组行为）。

---

## 集成测试

`//go:build integration`，用 testcontainers 拉起真实依赖：

- `internal/repository/integration_harness_test.go` —— 仓储层共享 harness（PostgreSQL）
- `internal/middleware/rate_limiter_integration_test.go` —— 限流器（Redis）
- `internal/server/routes/auth_rate_limit_integration_test.go` —— 认证限流

需要本地 Docker 可用。写新的集成测试时优先复用现有 harness，不要各自起容器。

---

## E2E 测试

`internal/integration/`（`//go:build e2e`）：`e2e_gateway_test.go`、`e2e_user_flow_test.go`、`e2e_helpers_test.go`。

```bash
cd backend
make test-e2e-local     # go test -tags=e2e -v -timeout=300s ./internal/integration/...
make test-e2e           # ./scripts/e2e-test.sh
```

CI 不跑 e2e，改动网关主链路时请本地跑一遍。

---

## 什么该测

- **必测**：计费/额度计算、认证与鉴权分支、调度与故障转移、错误码映射、软删除边界、并发槽位。
- **应测**：新 service 的主路径 + 关键失败路径；新 HTTP 端点的参数校验与权限。
- **不必测**：ent 生成代码、纯 DTO 字段拷贝。

---

## 反模式

- **同义反复测试**：把被测功能整体删掉后测试仍然通过 —— 说明它没测到东西
  （判定方法见 [`../guides/index.md`](../guides/index.md)）。
- **忘写 build tag** —— 测试永不执行。
- **在 unit 测试里连真实 DB/Redis** —— 应该用 stub/miniredis/sqlmock，或改标 integration。
- **从 integration/e2e 测试 import `internal/testutil`** —— 该包是 `unit` tag，编译不到。
- **改 interface 只补一个 stub** —— 其余 mock 会编译失败，一次性全补。
