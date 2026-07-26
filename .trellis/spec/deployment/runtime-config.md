# Runtime Config

> 配置从哪来、环境变量怎么推导、新增配置项要改哪几处，以及镜像构建与编排约定。
> 源文件：`backend/internal/config/config.go`、`backend/internal/setup/setup.go`、
> `deploy/config.example.yaml`、`deploy/docker-compose.yml`、根 `Dockerfile`。

---

## 配置来源与优先级

三层，从低到高：**内置默认值 → `config.yaml` → 环境变量**；
其中一部分设置在启动后由**数据库中的管理端设置**接管（见 [Reverse Proxy](./reverse-proxy.md#客户端-ip-信任模型最容易配错的部分) 的转发头列表）。

`config.go` 的 `load()` 按固定顺序搜索 `config.yaml`（找到第一个即停）：

```go
viper.SetConfigName("config")
viper.SetConfigType("yaml")
if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
    viper.AddConfigPath(dataDir)      // 1. DATA_DIR（最高优先级）
}
viper.AddConfigPath("/app/data")      // 2. Docker 数据目录
viper.AddConfigPath(".")              // 3. 当前目录
viper.AddConfigPath("./config")       // 4. ./config
viper.AddConfigPath("/etc/sub2api")   // 5. 系统目录
```

- **配置文件缺失是合法的**：`viper.ConfigFileNotFoundError` 被容忍，走默认值 + 环境变量。
  其他读取错误（YAML 语法错误等）直接返回失败。
- 数据目录的判定在 `setup.GetDataDir()`：`DATA_DIR` → `/app/data`（存在且可写）→ 兜底。
  Docker 镜像里 `/app/data` 是卷挂载点，`config.yaml` 由 `AUTO_SETUP` 生成在这里。
- `Load()` 要求 `jwt.secret` 非空；`LoadForBootstrap()` 允许为空——
  启动早期还没有数据库里的密钥，填入后会**再校验一次**。
  新增「必须非空」的配置项时注意别破坏这个两阶段流程。

---

## 环境变量命名

```go
viper.AutomaticEnv()
viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
```

配置 key 的点号换成下划线、全大写，就是环境变量名：

| 配置 key | 环境变量 |
|----------|---------|
| `server.port` | `SERVER_PORT` |
| `database.dbname` | `DATABASE_DBNAME` |
| `gateway.max_body_size` | `GATEWAY_MAX_BODY_SIZE` |
| `gateway.image_concurrency.overflow_mode` | `GATEWAY_IMAGE_CONCURRENCY_OVERFLOW_MODE` |

### ★ 最容易踩的坑：只写在 example 文件里的 key 拿不到环境变量

`viper.Unmarshal` 只解码 `AllKeys()` 返回的 key，而 `AllKeys()` 是
**`SetDefault` ∪ 配置文件实际存在的 key ∪ 显式 `BindEnv` 的 key** 的并集。

> `AutomaticEnv` 能覆盖已经在这个并集里的 key，但**永远不会新增 key**；
> 而 `viper_bind_struct` 逃生通道在 `-tags embed` 构建下被编译掉了。

后果：一个只出现在 `deploy/config.example.yaml`、没有任何默认值的 key，
在**纯环境变量部署**（也就是 `deploy/docker-compose.yml` 的方式）下，
值会被读出来然后**静默丢弃**，用户拿到零值且没有任何警告。

`config.go` 的 `setEnvReachableDefaults()` 就是为此存在的——给这类 key 注册零值默认，
让它变成「可被环境变量寻址」。**新增配置项时必须做这件事**：

```go
func setEnvReachableDefaults() {
    viper.SetDefault("gateway.session_idle_timeout_minutes", 0)
    viper.SetDefault("update.proxy_url", "")
    ...
}
```

三条注册规则：

1. **默认注册零值**，不要注册 example 里的示例值——缺失 key 本来就 unmarshal 成零值，
   注册零值保持行为不变，只是让 key 可寻址。需要更丰富的默认值时，
   在 unmarshal **之后**由各子系统自己应用（既有做法）。
2. **有效默认值为 `true` 的开关是例外**，必须注册 `true`。
   例：`gateway.openai_scheduler.sticky_escape_enabled` 用 `viper.IsSet` 做守卫，
   注册 `false` 会让 `IsSet` 恒为真，从而**永久关掉**该特性。
3. **需要区分「显式配置」与「未配置」的 key 不能 `SetDefault`**——
   `viper.IsSet` 也会把注册过的默认值报成已设置。
   `server.trusted_proxies` 和 `security.forwarded_client_ip_headers` 属于这类，
   改用 `viper.BindEnv` 记录可达性（不影响变量缺失时的 `IsSet`）。

### 手工解析的环境变量

逗号分隔的列表值在 `load()` 里用 `os.LookupEnv` + `normalizeStringSlice(strings.Split(v, ","))` 处理：

| 环境变量 | 对应 key | 说明 |
|---------|---------|------|
| `SERVER_TRUSTED_PROXIES` | `server.trusted_proxies` | 逗号分隔 CIDR/IP |
| `SECURITY_FORWARDED_CLIENT_IP_HEADERS` | `security.forwarded_client_ip_headers` | 逗号分隔头名；**显式设为空会清空 YAML 值** |
| `ENABLE_SERVER_TIMING` | `server.enable_server_timing` | 单独 `BindEnv`，变量名不遵循推导规则 |

`os.LookupEnv` 而不是 `os.Getenv`：需要区分「设为空字符串」（=清空）和「未设置」（=用 YAML）。

---

## AUTO_SETUP 引导

Docker 部署**必须** `AUTO_SETUP=true`（`setup.AutoSetupEnabled()` 接受 `true` / `1` / `yes`）。
`AutoSetupFromEnv()` 从环境变量生成 `config.yaml` 并创建管理员，用的是**独立的一套变量名**
（`setup` 包的 `getEnvOrDefault`，不走 viper）：

| 变量 | 默认值 |
|------|--------|
| `DATABASE_HOST` / `DATABASE_PORT` / `DATABASE_USER` / `DATABASE_PASSWORD` / `DATABASE_DBNAME` / `DATABASE_SSLMODE` | `localhost` / `5432` / `postgres` / 空 / `sub2api` / `disable` |
| `REDIS_HOST` / `REDIS_PORT` / `REDIS_USERNAME` / `REDIS_PASSWORD` / `REDIS_DB` / `REDIS_ENABLE_TLS` | `localhost` / `6379` / 空 / 空 / `0` / `false` |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | `admin@sub2api.local` / 空 |
| `SERVER_HOST` / `SERVER_PORT` / `SERVER_MODE` | `0.0.0.0` / `8080` / `release` |
| `JWT_SECRET` / `JWT_EXPIRE_HOUR` | 空 / `24` |
| `SETUP_MIGRATION_TIMEOUT_SECONDS` | `0` |
| `TZ`（回退 `TIMEZONE`） | `Asia/Shanghai` |

**必须显式固定的两个密钥**（`docker-compose.yml` 里有醒目注释）：

- `JWT_SECRET` —— 不固定则每次重启随机生成，**所有登录会话失效**。
- `TOTP_ENCRYPTION_KEY` —— 不固定则**已绑定的 2FA 全部失效**。

`TZ` 影响进程内所有时间运算（统计聚合、账单周期、清理任务），
应用与 postgres / redis 三个容器要**设成同一个值**。

---

## run_mode

`run_mode: "standard" | "simple"`，`config.NormalizeRunMode()` 做小写 + trim，
**非法值静默回落到 `standard`**（不报错）。

`simple` 模式的行为差异（不是「功能少一点」，是启动与路由行为都变）：

- `repository/ent.go` 启动时补齐各平台默认分组：
  `anthropic|openai|gemini` 确保存在 `<platform>-default`；`antigravity` 要求 ≥2 个未软删分组；
  并调整管理员并发（`ensureSimpleModeAdminConcurrency`）。
- 计费类端点直接返回 404（`gateway_key_billing.go`：`Billing information is not supported in simple mode`）。

**新增功能时要判断它在 simple 模式下是否成立**，需要屏蔽就参照上面的 404 写法，
不要让功能在缺少分组/计费数据的前提下崩掉。

---

## 镜像构建

根 `Dockerfile` 是三阶段，**顺序不能倒**（后端要嵌入前端产物）：

| 阶段 | 基础镜像 | 做什么 |
|------|---------|--------|
| frontend-builder | `node:24-alpine` | `corepack prepare pnpm@9` → `pnpm run build`，产物落到 `backend/internal/web/dist`；同时拷 `docs/legal/` |
| backend-builder | `golang:1.26.5-alpine` | 从上一阶段取 `internal/web/dist`，`CGO_ENABLED=0 go build -tags embed -ldflags="-s -w -X main.Version=... -X main.BuildType=release" -trimpath` |
| runtime | `alpine:3.21` | 从 `postgres:18-alpine` 取 `pg_dump`/`psql`/`libpq`；uid/gid `1000` 的 `sub2api` 用户；`/app/data`；`EXPOSE 8080`；healthcheck 打 `/health` |

- **`-tags embed` 是必须的**：没有它 `internal/web/embed_off.go` 生效，二进制不含前端。
  相关文件：`embed_on.go`（`//go:embed all:dist`）、`embed_off.go`、`html_cache.go`、`static_cache.go`。
- 构建参数：`NODE_IMAGE` / `GOLANG_IMAGE` / `ALPINE_IMAGE` / `POSTGRES_IMAGE`（换源用）、
  `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=sum.golang.google.cn`、`NPM_CONFIG_REGISTRY`、
  `VERSION` / `COMMIT` / `DATE`（打进 ldflags）。
- `deploy/docker-entrypoint.sh`：以 root 起，`chown -R sub2api:sub2api /app/data`
  （失败不阻塞，`|| true`），然后 `exec su-exec sub2api "$0" "$@"` 降权重入，最后 `exec "$@"`。
  **容器内不要以 root 跑应用**。

---

## 编排约定（`deploy/docker-compose.yml`）

- 镜像：`weishaw/sub2api:latest` + `postgres:18-alpine` + `redis:8-alpine`。
- 三个服务都 `restart: unless-stopped`、`ulimits.nofile` 软硬都 `100000`
  （长连接网关会开大量 fd）。
- **postgres / redis 不对宿主机暴露端口**，只走内部网络 `sub2api-network`；
  调试时才临时加 `ports: ["127.0.0.1:5433:5432"]`。
- 应用只暴露 `${BIND_HOST:-0.0.0.0}:${SERVER_PORT:-8080}:8080`；
  前面有反代时把 `BIND_HOST` 收到 `127.0.0.1`。
- 健康检查：应用 `wget -q -T 5 -O /dev/null http://localhost:8080/health`（30s/10s/3，start_period 30s）；
  postgres `pg_isready`；redis `redis-cli ping`（靠 `REDISCLI_AUTH` 免密码明文）。
- `POSTGRES_PASSWORD` 用 `${POSTGRES_PASSWORD:?...}` 强制必填。

### 两个已被注释固化的坑，改编排时不要还原

1. **`PGDATA=/var/lib/postgresql/data` 必须显式设置。**
   `postgres:18-alpine` 默认 `PGDATA=/var/lib/postgresql/18/docker`，落在镜像声明的匿名卷里。
   不设置的话，即使把 `postgres_data` 挂到 `/var/lib/postgresql/data`，数据也不进命名卷，
   `docker compose down/up` 之后 `initdb` 重新初始化 → **数据丢失**。
2. **Redis 的 `command` 每行末尾的 `\` 不能删。**
   `command: >` 折叠成给内层 `sh -c` 的单个带引号脚本，compose 会保留换行；
   少一个续行符，`redis-server` 就会**不带任何参数**启动，
   `--save` / `--appendonly` / `--appendfsync` 全部失效。

变体：`docker-compose.dev.yml`（本地开发）、`.local.yml`、`.standalone.yml`（不带数据库）。

---

## 新增配置项的落地清单

改一个配置项要同时动的地方：

- [ ] `backend/internal/config/config.go` 的结构体加字段，带 `mapstructure` tag（必要时加 `yaml` tag）
- [ ] `setDefaults()` 给默认值；**没有默认值就必须在 `setEnvReachableDefaults()` 注册零值**
- [ ] 需要区分「显式配置 vs 未配置」→ 不用 `SetDefault`，改 `BindEnv`
- [ ] 值是列表且要支持环境变量 → 在 `load()` 里加 `os.LookupEnv` + 逗号切分
- [ ] `deploy/config.example.yaml` 补上条目与中英双语注释（这是面向用户的唯一全量文档）
- [ ] 需要 Docker 部署可配 → `deploy/docker-compose.yml` 的 `environment` 加一行
- [ ] 校验逻辑放 `validate()`，错误信息要能指出是哪个 key
- [ ] 运行时可改（管理端设置）→ 走数据库设置 + 快照读取，不要直接读 viper

---

## 常见错误

- **只在 `config.example.yaml` 里加了 key** —— 纯环境变量部署下静默为零值。见上面的 `AllKeys()` 并集规则。
- **给「有效默认为 true」的开关注册 `false` 默认值** —— `viper.IsSet` 恒真，特性被永久关闭。
- **给 `server.trusted_proxies` 之类加 `SetDefault`** —— 破坏「是否显式配置过」的判定，
  连带影响 [Reverse Proxy](./reverse-proxy.md#升级行为) 里描述的升级行为。
- **忘了 `-tags embed`** —— 二进制不含前端，访问首页 404。
- **`JWT_SECRET` / `TOTP_ENCRYPTION_KEY` 留空** —— 重启后会话与 2FA 全部失效。
- **漏设 `PGDATA`** —— 重启后数据库被重新 initdb。
- **应用与数据库容器 `TZ` 不一致** —— 统计聚合与账单周期错位。
- **在 `Load()` 里给新字段加「必须非空」校验** —— 可能打断 `LoadForBootstrap()` 的两阶段启动。
