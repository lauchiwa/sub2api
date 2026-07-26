# Database Guidelines

> Ent ORM 用法、软删除语义、事务写法与 SQL 迁移流程。

---

## Ent Schema 定义

实体定义在 `backend/ent/schema/*.go`，这是 `ent/` 下唯一可手写的目录。

标准写法（`ent/schema/user.go`）：

```go
type User struct{ ent.Schema }

// 显式指定表名，不依赖 Ent 的默认复数推导
func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "users"}}
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},        // created_at / updated_at
		mixins.SoftDeleteMixin{},  // deleted_at
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").MaxLen(255).NotEmpty(),
		// 金额一律 decimal(20,8)，禁止用裸 float 落库
		field.Float("balance").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		// 状态/角色的默认值取自 domain 常量，不要写字面量
		field.String("role").MaxLen(20).Default(domain.RoleUser),
		field.String("status").MaxLen(20).Default(domain.StatusActive),
	}
}
```

规则：

- 表名一律通过 `entsql.Annotation{Table: "..."}` 显式声明。
- 金额字段用 `SchemaType` 指定 `decimal(20,8)`（`balance`、`frozen_balance` 等）。
- 枚举型字符串字段的 `Default()` 必须引用 `internal/domain` 常量，不写字面量。
- 字符串字段必须给 `MaxLen`。
- 新实体默认带 `TimeMixin`；需要回收站 / 可恢复语义时再加 `SoftDeleteMixin`。

### 改 schema 后必须重新生成

```bash
cd backend
go generate ./ent
git add ent/            # 生成代码是版本库的一部分，必须一起提交
```

漏掉这一步的典型症状是「改了 schema 但代码不生效」。

---

## 软删除语义

`ent/schema/mixins/soft_delete.go` 用 Ent 的 Interceptor + Hook 实现：

- 所有查询默认自动追加 `deleted_at IS NULL`。
- `Delete` 被改写成 `UPDATE SET deleted_at = NOW()`，不做物理删除。
- 需要绕过时用 `mixins.SkipSoftDelete(ctx)`：

```go
// 查询含已删除记录
users, err := client.User.Query().All(mixins.SkipSoftDelete(ctx))

// 真正物理删除
err := client.User.DeleteOneID(id).Exec(mixins.SkipSoftDelete(ctx))
```

### 唯一约束必须用部分索引

软删除后要允许同值重用，唯一约束一律写成 `WHERE deleted_at IS NULL` 的 partial unique index，
定义在 SQL 迁移里，而不是 Ent 的 `index.Fields(...).Unique()`。

参考 `migrations/016_soft_delete_partial_unique_indexes.sql`，
以及 `ent/schema/user.go` 中对应字段上方的注释说明。

**反模式**：给带软删除的实体加普通唯一索引 —— 用户注销后无法用同邮箱重新注册。

---

## 事务

统一使用 Ent 事务，**不要**手工用 `*sql.Tx` 构造 ent client（会触发 `ExecQuerier` 断言错误）。

嵌套事务的标准处理（见 `internal/repository/user_repo.go` 的 `create`）：

```go
tx, err := r.client.Tx(ctx)
if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
	return err
}

var txClient *dbent.Client
txCtx := ctx
if err == nil {
	defer func() { _ = tx.Rollback() }()
	txClient = tx.Client()
	txCtx = dbent.NewTxContext(ctx, tx)
} else {
	// 已在外部事务中：复用当前事务 client，由调用方负责提交/回滚
	if existingTx := dbent.TxFromContext(ctx); existingTx != nil {
		txClient = existingTx.Client()
	} else {
		txClient = r.client
	}
}
```

要点：

- `ErrTxStarted` 表示已处于外层事务，必须复用而不是新开一个。
- 自己开的事务用 `defer tx.Rollback()` 兜底，成功路径显式 `Commit()`。
- 事务上下文通过 `dbent.NewTxContext(ctx, tx)` 向下传递。
- 涉及多表原子更新（用户 + 允许分组、订单 + 余额）必须包在同一事务内。

---

## SQL 迁移

`backend/migrations/` 下手写 SQL，命名 `NNN_描述.sql`（三位序号，当前已有 242 个文件）。

规则：

- **迁移文件一旦合入就不可修改**；修正要新增后续序号的文件
  （参考 `006_fix_invalid_subscription_expires_at.sql`、`006b_guard_users_allowed_groups.sql`）。
- 迁移必须可重复执行（`IF NOT EXISTS` / `IF EXISTS` / 幂等 DML）。
- 破坏性变更拆两步：先加兼容结构，后续版本再删旧结构
  （参考 `006_add_users_allowed_groups_compat.sql` → `014_drop_legacy_allowed_groups.sql`）。
- Ent schema 与 SQL 迁移成对提交：schema 描述结构，迁移负责已有库的演进
  和 Ent 表达不了的部分（partial index、数据回填、约束修正）。

---

## 查询与分页

- 分页统一走 `internal/pkg/pagination` 的 `PaginationParams{Page, PageSize, SortBy, SortOrder}`。
- 排序字段必须白名单校验，禁止把用户输入直接拼进 `ORDER BY`。
- 需要原生 SQL 时通过 repository 内的 `sqlExecutor`（见 `internal/repository/user_repo.go` 的 `sql` 字段）
  或 `entsql`，一律使用参数占位符，禁止字符串拼接。
- 聚合类查询放独立文件，例如 `internal/repository/dashboard_aggregation_repo.go`、
  `channel_repo_account_stats_pricing.go`。
- 大范围扫描要有边界（LIMIT / 时间窗 / 游标），避免无界全表扫描。

---

## Redis 缓存

Redis 只能出现在 `internal/repository/`（由 `.golangci.yml` 的 depguard 强制）。

- 缓存实现统一命名 `*_cache.go`：`gateway_cache.go`、`billing_cache.go`、`api_key_cache.go`、
  `dashboard_cache.go`、`identity_cache.go`、`concurrency_cache.go` 等。
- 接口在 `internal/service/` 定义（如 `service.ConcurrencyCache`、`service.SessionLimitCache`）。
- TTL 等参数从 `config.Config` 注入，通过 `internal/repository/wire.go` 的 `ProvideXxxCache` provider 传入，
  不要在缓存实现内部硬编码。

---

## 数据库方言

生产用 PostgreSQL（Docker 镜像 `postgres:18-alpine`）；无 CGO 场景可用 `modernc.org/sqlite`。
`SchemaType` 目前只显式声明 `dialect.Postgres`；新增方言敏感字段时注意两边差异，
尤其是 `timestamptz` 与 `decimal` 的精度行为。

---

## 常见错误

| 症状 | 根因 | 处理 |
|------|------|------|
| 改了 schema 代码不生效 | 忘记 `go generate ./ent` | 重新生成并提交 `ent/` |
| 注销用户无法用原邮箱注册 | 用了普通唯一索引而非 partial unique index | 改用 `WHERE deleted_at IS NULL` 部分索引 |
| `ExecQuerier` 断言错误 | 用 `*sql.Tx` 手工构造 ent client | 改用 `client.Tx(ctx)` |
| 事务嵌套后数据不一致 | 忽略了 `ErrTxStarted` 直接新开事务 | 复用 `dbent.TxFromContext(ctx)` |
| 查到已删除数据 | 误用了 `SkipSoftDelete` | 只在回收站/物理清理路径使用 |
