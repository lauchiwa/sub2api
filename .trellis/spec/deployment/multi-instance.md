# 多实例部署规范

> 多个 sub2api 进程共享同一套 PostgreSQL + Redis 时的约束。适用于水平扩容、
> 蓝绿发布，以及"多台开发机共用一套后端数据"这类场景。
> 配置项本身的加载规则见 [`./runtime-config.md`](./runtime-config.md)。

---

## 结论先行

Sub2API **是按多实例设计的**：迁移有 PostgreSQL advisory lock、后台任务有 Redis
leader lock、缓存失效有 Redis pub/sub、OAuth 刷新有按账号粒度的分布式锁。
把 `database.*` 和 `redis.*` 指向同一套后端，多个实例即可共享全部账号与业务数据。

但有 **一项配置不显式设置就会造成不可逆的数据损坏**，见下一节。

---

## 一、必须跨实例一致的配置

### `TOTP_ENCRYPTION_KEY` —— 唯一的必做项

```go
// internal/config/config.go:1784
cfg.Totp.EncryptionKey = strings.TrimSpace(cfg.Totp.EncryptionKey)
if cfg.Totp.EncryptionKey == "" {
    key, err := generateJWTSecret(32)
    ...
    cfg.Totp.EncryptionKey = key
    cfg.Totp.EncryptionKeyConfigured = false
    slog.Warn("TOTP encryption key auto-generated. Consider setting a fixed key for production.")
}
```

留空时**每个进程各自随机生成一把 AES-256 密钥**，只打一条 `slog.Warn`，不会阻止启动。
它也**不走** `security_secrets` 表的 bootstrap —— `internal/repository/security_secret_bootstrap.go:20`
只登记了 `securitySecretKeyJWT = "jwt_secret"` 一个 key。

而这把 key 不止管 TOTP。`internal/repository/aes_encryptor.go:23` 用它构造全局
`service.SecretEncryptor`：

| 使用方 | 加密对象 |
|---|---|
| `service/totp_service.go` | 用户 2FA 密钥 |
| `service/channel_monitor_service.go` | 渠道监控的上游 API Key |
| `service/backup_service.go` | 备份内的敏感字段 |
| `service/openai_live_attestation.go` | Live attestation 凭据 |

**后果**：实例 A 加密写入的密文，实例 B 用另一把 key 解不开。
`channel_monitor_service.go:154` 的注释记录了这条路径踩过的坑——解密失败时
APIKey 曾被静默清空。这不是"重启后失效"，是**已落库数据永久读不回来**。

```bash
# 生成一次，两边写同一个值
openssl rand -hex 32
# .env / config.yaml 都可以，环境变量名：
TOTP_ENCRYPTION_KEY=<64 位 hex>
```

### `JWT_SECRET` —— 不用管

第一个启动的实例把它写进 `security_secrets` 表，后续实例自动读同一个，
登录态天然互通：

```go
// internal/repository/security_secret_bootstrap.go:37
storedSecret, err := createSecuritySecretIfAbsent(ctx, client, securitySecretKeyJWT, cfg.JWT.Secret)
// :48  配置为空时改为生成并落库
secret, created, err := getOrCreateGeneratedSecuritySecret(ctx, client, securitySecretKeyJWT, 32)
```

显式配了 `JWT_SECRET` 也可以（会以配置值入库），但**多实例配了就必须配成一样的**，
否则先启动的那个值入库、后启动的用自己的值签发，token 互不认。不配反而更省心。

### 其余必须一致的项

| 配置 | 环境变量 | 原因 |
|---|---|---|
| `database.*` | `DATABASE_HOST` / `DATABASE_PORT` / `DATABASE_USER` / `DATABASE_PASSWORD` / `DATABASE_DBNAME` | 指向同一个库才叫共享 |
| `redis.host` / `redis.port` / `redis.db` | `REDIS_HOST` / `REDIS_PORT` / `REDIS_DB` | leader lock 与 pub/sub 依赖同一个 Redis 实例和**同一个 DB 编号** |
| `run_mode` | `RUN_MODE` | `standard` / `simple` 决定是否注册计费路由，混用会让同一份数据在不同实例上表现不一致 |

> `redis.db` 不同 = 换了命名空间，leader lock 互不可见、pub/sub 收不到对方消息，
> 表现为"看起来正常但定时任务双跑、缓存不失效"。这是最难查的一类配置错误。

---

## 二、并发安全机制清单

### 2.1 数据库迁移：PostgreSQL Advisory Lock

```go
// internal/repository/migrations_runner.go:48
// migrationsAdvisoryLockID 是用于序列化迁移操作的 PostgreSQL Advisory Lock ID。
const migrationsAdvisoryLockID int64 = 694208311321144027
const migrationsLockRetryInterval = 500 * time.Millisecond
```

多实例同时冷启动是安全的：先拿到锁的执行迁移，其余每 500ms 重试，锁释放后跳过已应用项。
锁绑在一个独占连接上（`db.Conn(ctx)`），进程崩溃时连接断开由 PostgreSQL 自动释放，
不会留下死锁。

### 2.2 后台定时任务：Redis Leader Lock

```go
// internal/repository/leader_lock_cache.go
const leaderLockKeyPrefix = "leader:lock:"
// SetNX 抢锁 + 校验 owner 的 Lua 脚本释放，避免释放别人的锁
func (c *leaderLockCache) TryAcquireLeaderLock(ctx, key, owner string, ttl time.Duration) (bool, error) {
    return c.rdb.SetNX(ctx, leaderLockKeyPrefix+key, owner, ttl).Result()
}
```

owner 是进程启动时生成的 `uuid.NewString()`（如 `upstream_billing_probe.go:216`）。
受保护的任务：

| Lock Key | TTL | 服务 |
|---|---|---|
| `dashboard:aggregation:leader` | 5m | `dashboard_aggregation_service.go` |
| `ops:aggregation:daily:leader` | 10m | `ops_aggregation_service.go` |
| `ops:aggregation:hourly:leader` | 15m | 同上 |
| `ops:alert:evaluator:leader` | 90s | `ops_alert_evaluator_service.go` |
| `ops:metrics:collector:leader` | 90s | `ops_metrics_collector.go` |
| `payment:order:expiry:leader` | 3m | `payment_order_expiry_service.go` |
| `subscription:expiry:reminder:leader` | 5m | `subscription_expiry_service.go` |
| `upstream:billing:probe:leader` | 2m | `upstream_billing_probe.go` |
| `ollama:cloud:usage:leader` | 2m | `ollama_cloud_usage.go` |
| `ops:cleanup:leader` | 30m（默认） | `ops_cleanup_service.go` |
| `ops:scheduled_reports:leader` | 5m（默认） | `ops_scheduled_report_service.go` |

末两项标"默认"是因为它们的 key 与 TTL 可被 `OpsDistributedLockSettings` 运行时覆盖
（`ops_scheduled_report_service.go:802`、`ops_alert_evaluator_service.go:178`），
其余为编译期常量。

**新增周期性任务时，先判断它是否"全局只应执行一次"**。是的话必须走
`service.LeaderLockCache`，照抄上表任一服务的写法。TTL 的选取仓库里已有明文约定：

```go
// internal/service/dashboard_aggregation_service.go:24
// dashboardAggregationLeaderLockTTL must exceed the job's worst-case runtime
```

锁提前过期会让第二个实例在第一个还没跑完时并行进入——这正是 leader lock 要避免的情况。

### 2.3 跨实例缓存失效：Redis Pub/Sub

进程内缓存靠 Redis 频道广播失效，避免"A 改了设置 B 还在用旧值"：

| 频道 | 失效对象 | 出处 |
|---|---|---|
| `auth:cache:invalidate` | API Key / 鉴权缓存 | `repository/api_key_cache.go:99` |
| `subscription:cache:invalidate` | 订阅与计费缓存 | `repository/billing_cache.go:261` |
| `tls_fingerprint_profiles_updated` | TLS 指纹配置 | `repository/tls_fingerprint_profile_cache.go:94` |
| `error_passthrough_rules_updated` | 错误透传规则 | `repository/error_passthrough_cache.go:100` |
| `sub2api:prompt_guard:config:invalidate` | 安全审计 prompt 配置 | `securityaudit/prompt_config_store.go:280` |

**新增进程内缓存时，如果它的数据能被管理端修改，必须同时接一个失效频道。**
只写 `sync.Map` 不广播，在单实例上测不出问题，多实例上表现为"改了设置部分请求不生效"。

### 2.4 OAuth Token 刷新：按账号粒度的分布式锁

```go
// internal/repository/gemini_token_cache.go:41
func (c *geminiTokenCache) AcquireRefreshLock(ctx context.Context, cacheKey string, ttl time.Duration) (bool, error) {
    key := fmt.Sprintf("%s%s", oauthRefreshLockKeyPrefix, cacheKey)
    return c.rdb.SetNX(ctx, key, 1, ttl).Result()
}
```

调用方：`gemini_token_provider.go:95`、`openai_token_provider.go:209`、
`claude_token_provider.go:107`、`antigravity_token_provider.go:124`、
`vertex_service_account.go:166`（均 30s TTL），以及通用的 `oauth_refresh_api.go:195`
（TTL 来自 `api.lockTTL`，并由 `clampRefreshAttemptToLockLease` 保证单次刷新
超时不超过租约，见 `token_refresh_service.go:790`）。

这里用**按账号锁**而不是 leader lock 是正确选择：刷新是按账号触发的，
不同账号应当能并行。很多 OAuth 提供方刷新时会轮换 refresh token，
两个实例同时刷同一个账号会互相作废——这条路径已被覆盖。

### 2.5 系统级操作：DB 互斥锁

`service/system_operation_lock_service.go` 基于 `IdempotencyRepository`
（`idempotency_records` 表）实现，带续租循环（`renewLoop`）。
使用方是 `handler/admin/system_handler.go`，即**在线更新 / 回退**。

> 注意语义边界：这把锁保证同一时刻只有一个更新操作在跑，但**在线更新替换的是
> 本机二进制**（`service/update_service.go:561`、`:583` 的 `os.Create(destPath)`）。
> 从 Web UI 触发更新，只会更新恰好处理该请求的那个实例。多实例下版本升级需要逐台执行。

---

## 三、没有跨实例互斥的后台任务

以下任务**会在每个实例上各跑一遍**。判断标准是"重复执行是否有害"：

| 服务 | 重复执行的后果 | 处理建议 |
|---|---|---|
| `backup_service.go` | ⚠️ **每台各备份一份**。cron 表达式来自 DB 设置 `backup_schedule`，两台读到同一个计划；进程内只有 `cronMu` / `opMu` 两把 `sync.Mutex`，不跨实例 | 只在一台上启用定时备份，或改造为走 leader lock |
| `account_expiry_service.go` | 按时间条件的 UPDATE，幂等 | 无需处理，仅多一次扫描开销 |
| `proxy_expiry_service.go` | 同上 | 同上 |
| `idempotency_cleanup_service.go` | `repo.DeleteExpired(...)`，幂等 | 同上 |
| `audit_log_service.go` | 刷写的是**本实例**的缓冲区 | 本就应该每实例执行 |
| `service/pricing_service.go` | 各自从上游同步定价到本地 `Pricing.DataDir` | 本就应该每实例执行 |

`internal/pkg/*/oauth.go`、`openai_ws_pool.go`、`gateway_*` 里的 ticker 属于
"每连接 / 每请求"生命周期，天然是每实例的，不在此列。

---

## 四、每实例独立、不会自动同步的状态

| 状态 | 位置 | 说明 |
|---|---|---|
| 自定义页面 | `{DATA_DIR}/pages/*.md`（`handler/page_handler.go:29`） | 管理端上传的是**文件不是 DB 记录**，各实例各存各的 |
| 定价缓存 | `{pricing.data_dir}/model_pricing.json` + `.sha256`（`service/pricing_service.go:1047`，注意同名的 `repository/pricing_service.go` 不是这个） | 从上游同步的只读缓存，会自愈，无需干预 |
| 应用二进制 | 本机文件系统 | 在线更新只更新当前实例，见 2.5 |

自定义页面若需要一致，用共享卷或部署时同步 `{DATA_DIR}/pages/`。

---

## 五、基础设施版本下限

| 组件 | 声明下限 | 仓库编排用的版本 | 说明 |
|---|---|---|---|
| PostgreSQL | **15+**（`README.md:225`） | `postgres:18-alpine`（`deploy/docker-compose.yml:204`） | 代码无版本探测；242 个迁移文件未使用 16/17/18 专属语法 |
| Redis | **7+**（`README.md:226`） | `redis:8-alpine`（`deploy/docker-compose.yml:250`） | 只用到 ZAdd / Pipeline / TxPipeline / SetNX / Publish / Subscribe / Eval |

**Redis 是必需依赖，不是可选缓存。** 连接失败直接启动失败：

```go
// internal/setup/cli.go:158
return fmt.Errorf("redis connection failed: %w", err)
```

**唯一与 PostgreSQL 版本强相关的是备份**：`backup_service.go` 调用外部 `pg_dump`，
而 `pg_dump` 只能向下兼容。容器部署时镜像自带 `postgres:18-alpine` 的
`pg_dump`（`Dockerfile:132`），裸机部署时用的是宿主机的 client——
**本机 `pg_dump` 版本必须 ≥ 服务端版本**，否则备份报版本不匹配。

MySQL / SQLite 均不支持生产使用：`backend/go.mod` 生产依赖里只有
`github.com/lib/pq`，ent 生成代码的方言分支只判断 `dialect.Postgres`，
SQLite 仅出现在单元测试中。

---

## 六、网络与安全

多实例最常见的形态是实例与数据库不在同一台机器上，此时以下两条是硬要求。

### `sslmode` 默认值会静默降级

```go
// internal/config/config.go:1993
viper.SetDefault("database.sslmode", "prefer")
```

libpq 的 `prefer` 在服务端不支持 TLS 时**静默降级为明文**，不报错。
数据库跨网络访问时必须显式设为 `require` 起步，条件允许用 `verify-full`：

```bash
DATABASE_SSLMODE=verify-full
```

### Redis 必须设密码

`RedisConfig` 提供 `Password`（`redis.password` / `REDIS_PASSWORD`）与
`EnableTLS`（`redis.enable_tls` / `REDIS_ENABLE_TLS`），但默认均为空 / `false`
（`config.go:2006`、`:2013`）。裸奔的 Redis 是最常见的入侵入口之一。

### 首选内网而非公网暴露

`deploy/docker-compose.yml` 的既定做法是 **PostgreSQL 与 Redis 不对宿主机暴露端口**。
跨机部署时应沿用同一原则：用 VPC 内网、或 WireGuard / Tailscale 之类的虚拟内网
把实例与数据服务连起来，数据服务只监听内网地址。这比"暴露公网 + 加 TLS + 加密码"
的攻击面小得多，也省掉证书运维。

另外注意延迟成本：单次网关转发要串多轮 DB / Redis 往返，
跨公网的 RTT 会被往返次数放大。

---

## 七、上线检查清单

```bash
# 1. 生成并在所有实例写入同一个值（缺这条会永久损坏加密数据）
openssl rand -hex 32

# 2. 每个实例的 .env 至少包含
TOTP_ENCRYPTION_KEY=<同一个 64 位 hex>
DATABASE_HOST=... DATABASE_PORT=... DATABASE_USER=...
DATABASE_PASSWORD=... DATABASE_DBNAME=...
DATABASE_SSLMODE=require          # 跨网络必须，别用默认的 prefer
REDIS_HOST=... REDIS_PORT=... REDIS_DB=0   # DB 编号必须一致
REDIS_PASSWORD=...
RUN_MODE=standard                 # 所有实例保持一致
```

- [ ] `TOTP_ENCRYPTION_KEY` 所有实例一致，且**不是**空值（启动日志里搜
      `TOTP encryption key auto-generated` 应无命中）
- [ ] `REDIS_DB` 编号一致（不一致会导致 leader lock 与 pub/sub 静默失效）
- [ ] `DATABASE_SSLMODE` 非 `prefer`（跨网络时）
- [ ] `RUN_MODE` 一致
- [ ] 定时备份只在一个实例上启用
- [ ] `pg_dump` 版本 ≥ 数据库服务端版本
- [ ] 自定义页面目录 `{DATA_DIR}/pages/` 已同步或确认不需要

---

## 八、常见错误

| 现象 | 根因 |
|---|---|
| 上游账号 API Key 解密失败 / 被清空，2FA 全部失效 | `TOTP_ENCRYPTION_KEY` 未固定，各实例生成了不同的 key |
| 定时任务重复执行、告警重复推送 | `REDIS_DB` 编号不一致，leader lock 互不可见 |
| 管理端改了设置，部分请求仍用旧值 | 新增的进程内缓存没接 pub/sub 失效频道 |
| 登录后立即失效 / token 在实例间不互认 | 显式配置了 `JWT_SECRET` 但各实例值不同 |
| 每天产生多份重复备份 | 定时备份未做单实例限制（见第三节） |
| 更新后发现只有一台升级了 | 在线更新替换的是本机二进制，需逐台执行 |
| 同一份数据在不同实例上功能不一致（如计费接口 404） | `RUN_MODE` 混用了 `standard` 与 `simple` |
| 数据库连接看似正常但流量是明文 | `DATABASE_SSLMODE` 用了默认的 `prefer`，服务端未开 TLS 时静默降级 |

---

## 相关规范

- [`./runtime-config.md`](./runtime-config.md) —— 配置加载顺序、环境变量命名推导、
  `AllKeys()` 并集陷阱、`run_mode` 语义
- [`./reverse-proxy.md`](./reverse-proxy.md) —— 多实例前置负载均衡时的可信代理与
  真实客户端 IP 约定
- [`../backend/architecture.md`](../backend/architecture.md) —— repository / service
  分层与 wire 装配
