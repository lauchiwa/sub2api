# Quality Guidelines

> lint 规则、格式化约定、禁止模式与提交前检查清单。

---

## 工具链

| 工具 | 版本 / 配置 |
|------|-------------|
| Go | 1.26.5（CI 断言 `go version \| grep -q 'go1.26.5'`） |
| golangci-lint | **v2**（`backend/.golangci.yml` 的 `version: "2"`） |
| 格式化 | golangci-lint 的 `formatters.gofmt` |

本地安装与执行：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7
cd backend && golangci-lint run ./...
```

---

## 启用的 linter

`.golangci.yml` 用 `default: none` 后显式启用：

```
depguard  errcheck  gosec  govet  ineffassign  staticcheck  unused
```

### depguard —— 架构边界（最重要）

`service` 与 `handler` 禁止 import `internal/repository`、`gorm.io/gorm`、`redis/go-redis/v9`。
完整说明见 [Architecture](./architecture.md#depguard-硬边界ci-强制)。**不要往白名单加新文件。**

### errcheck

配置比默认严格：

- `check-type-assertions: true` —— 类型断言必须写 `v, ok := x.(T)` 形式。
- `disable-default-exclusions: true` —— 默认豁免名单被关闭。
- 仅豁免 `fmt.Print*` / `fmt.Fprint*` / `io.Copy(*bytes.Buffer)` / `io.Copy(os.Stdout)` / `ioutil.ReadFile`。

所以 `defer f.Close()` 之类要写成 `defer func() { _ = f.Close() }()`。

### staticcheck

`checks: all` 后仅关闭注释格式类：`ST1000`、`ST1003`、`ST1020`、`ST1021`、`ST1022`。
`initialisms` 列表已配置（`API`/`URL`/`TLS`/`ID`/…），命名时遵循：`APIKey` 而非 `ApiKey`。

### unused

`generated-is-used: true` **必须保持 true**（ent 生成 13 万+ 行代码，否则误报爆炸）。

### gosec

`severity: high` + `confidence: high`，并排除了 G101/G103/G104/G109/G115/G201/G202/G301/G302/G304/G306。
安全扫描另有独立 workflow：`.github/workflows/security-scan.yml`（govulncheck + gosec + pnpm audit，每周一跑）。

---

## 格式化约定

`formatters.gofmt` 配置了两点非默认行为：

```yaml
simplify: false          # 不做 gofmt -s 简化
rewrite-rules:
  - interface{} → any    # 强制用 any
  - a[b:len(a)] → a[b:]
```

**写 `interface{}` 会被改成 `any`**，直接写 `any`。

文档注释需满足 gofmt 的注释规则（近期有一次专门修正：commit `ef0ca5bdf`
"style: 修正 dotStrippedEmailExpr 注释以满足 gofmt 文档注释规则"）。

---

## 必须遵守的模式

- **依赖倒置**：数据访问接口定义在 `service`，实现在 `repository`。
- **构造函数注入**：所有依赖通过 `NewXxx(...)` 传入并注册到 wire ProviderSet；禁止包级可变全局状态
  （logger 单例除外）。
- **后台组件可停止**：带 goroutine / ticker 的 service 必须有 `Stop()` 并注册进 `cmd/server/wire.go` 的 `provideCleanup`。
- **常量集中**：状态、角色、平台名一律用 `internal/domain/constants.go`，禁止散落字面量。
- **context 传递**：所有跨层调用首参 `ctx context.Context`，不要用 `context.Background()` 覆盖调用方的 ctx。
- **敏感字段脱敏**：出参走 `internal/handler/dto` 的映射器；日志走 `internal/util/logredact`。

---

## 禁止的模式

| 禁止 | 原因 | 替代 |
|------|------|------|
| `service` / `handler` import `repository`、`redis`、`gorm` | 破坏分层，CI 直接失败 | 在 service 定义 interface，repository 实现 |
| 手改 `ent/` 下的生成代码 | 会被 `go generate` 覆盖 | 改 `ent/schema/` 后重新生成 |
| 手改 `cmd/server/wire_gen.go` | 会被 `go generate ./cmd/server` 覆盖 | 改 `wire.go` |
| `log.Printf` / `fmt.Println` 输出业务日志 | 绕过级别控制与 Ops sink | `logger.L()` / `logger.LegacyPrintf` |
| `interface{}` | gofmt rewrite 规则会改写 | `any` |
| 忽略错误返回值（裸 `defer f.Close()`） | errcheck 严格模式 | `defer func() { _ = f.Close() }()` |
| SQL 字符串拼接 | 注入风险 | 参数占位符 / ent builder |
| service 层依赖 `*gin.Context` | 业务层不应感知 HTTP | 参数化传入所需字段 |
| 无界表扫描 | 数据量增长后打爆 DB | 加 LIMIT / 时间窗 / 游标 |

---

## 测试要求

详见 [Testing Guidelines](./testing-guidelines.md)。底线：

- 新增 service 逻辑要有 `//go:build unit` 单测。
- 改动 HTTP 契约要更新 `internal/server/api_contract_test.go`。
- 改 interface 后所有 stub/mock 必须补全方法。

---

## 提交前检查清单

来自 `DEV_GUIDE.md` 与 CI（`.github/workflows/backend-ci.yml`）：

```bash
cd backend
go test -tags=unit ./...          # 或 make test-unit
go test -tags=integration ./...   # 或 make test-integration
golangci-lint run ./...
```

- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] `golangci-lint run ./...` 无新增问题
- [ ] 改了 `ent/schema/` → 已 `go generate ./ent` 并提交 `ent/`
- [ ] 改了 wire provider → 已 `go generate ./cmd/server` 并提交 `wire_gen.go`
- [ ] 改了 interface → 所有测试 stub 已补全
- [ ] 改了 `frontend/package.json` → `pnpm-lock.yaml` 已同步提交

Windows 上没有 `make`，直接用 Makefile 里的原始命令。

---

## Code Review 关注点

1. 分层边界有没有被绕过（尤其新加的 import）。
2. 新的后台组件是否注册了 cleanup。
3. 错误是否带 `Reason`、是否用 `response.ErrorFrom` 统一出口。
4. 日志是否脱敏、component 是否规范、热路径是否过度打日志。
5. 数据库改动是否 schema + migration 成对、软删除实体的唯一约束是否用了 partial index。
6. 网关端点的错误格式是否符合上游协议。
7. 新常量是否放进了 `internal/domain`，是否有重复定义。

AI 交叉评审结果需按 [`../guides/index.md`](../guides/index.md) 的验证规则复核，预期约 35% 误报率。
