# Sub2API Docker 打包与 Docker Compose 部署说明

## 一句话结论
- 当前仓库已经具备可交付的 Docker 化部署资产：根目录多阶段 `Dockerfile`、`deploy/.env.example`、以及 4 套 Compose 配置都已存在；OpenAI OAuth probe / revive 功能本身也已完成实现、测试、广域 unit 回归和项目内上下文记录，可视为已收口。

## 1. 适用范围
本文面向需要把 Sub2API 部署到自己 Linux 服务器上的运维或开发同事，覆盖以下场景：

1. **直接使用官方镜像部署**：适合快速上线。
2. **基于当前仓库自行构建镜像**：适合需要打自定义版本。
3. **Docker Compose 一体化部署**：内置 PostgreSQL + Redis。
4. **Compose Standalone 部署**：数据库与 Redis 由外部提供。
5. **本地开发构建部署**：从当前源码直接 `docker compose up --build`。

---

## 2. 仓库内现有 Docker 资产

### 2.1 关键文件
- `Dockerfile`：根目录多阶段构建，负责前端打包、后端编译、运行时镜像组装。
- `backend/Dockerfile`：后端单独构建镜像，能力较旧，默认 Go 1.25.7；如无特殊需要，**优先使用根目录 `Dockerfile`**。
- `deploy/docker-entrypoint.sh`：容器启动前修正 `/app/data` 权限，再降权到 `sub2api` 用户运行。
- `deploy/.env.example`：Compose 环境变量模板。
- `deploy/docker-compose.yml`：**命名卷版**，适合常规 Docker 主机。
- `deploy/docker-compose.local.yml`：**本地目录版**，适合需要迁移/备份整个部署目录的场景，推荐优先使用。
- `deploy/docker-compose.standalone.yml`：仅启动 Sub2API，自带数据卷；PostgreSQL/Redis 需外部提供。
- `deploy/docker-compose.dev.yml`：本地源码构建调试版。

### 2.2 推荐选择
- **生产部署首选**：`deploy/docker-compose.local.yml`
  - 优点：数据都落在 `deploy/data`、`deploy/postgres_data`、`deploy/redis_data`，迁移和备份最简单。
- **简单快速部署**：`deploy/docker-compose.yml`
  - 优点：命名卷更省心；缺点是迁移时需要额外 Docker volume 操作。
- **已有托管数据库/缓存**：`deploy/docker-compose.standalone.yml`
- **本地修改代码验证**：`deploy/docker-compose.dev.yml`

---

## 3. 前置要求

### 3.1 宿主机要求
建议至少满足：
- Linux x86_64 主机
- 已安装 Docker Engine 24+
- 已安装 Docker Compose v2（`docker compose` 子命令）
- 能访问镜像仓库（如拉取较慢，建议切国内加速）

### 3.2 需要开放的端口
默认涉及：
- `8080/tcp`：Sub2API Web / API 入口
- `5432/tcp`：PostgreSQL（Compose 默认映射到宿主机，若不需要外部访问可自行收紧）
- `6379/tcp`：Redis（Compose 默认映射到宿主机，若不需要外部访问可自行收紧）

> 建议生产环境按需收敛 PostgreSQL/Redis 暴露范围，避免直接对公网开放。

---

## 4. 镜像构建说明

## 4.1 推荐：使用根目录多阶段 Dockerfile
根目录 `Dockerfile` 会：
1. 用 Node 24 构建前端；
2. 用 Go 1.26.2 编译后端；
3. 把前端静态资源嵌入后端产物；
4. 生成最终运行镜像。

在仓库根目录执行：

```bash
docker build -t sub2api:local .
```

如需指定构建元信息：

```bash
docker build \
  --build-arg VERSION=$(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD) \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t sub2api:local .
```

### 4.2 验证镜像

```bash
docker image ls | grep sub2api
docker run --rm sub2api:local --help
```

### 4.3 关于 `backend/Dockerfile`
`backend/Dockerfile` 目前是后端单体构建方式，未覆盖根目录多阶段构建的前端嵌入流程，且基线 Go 版本较旧。除非你明确只想做后端镜像实验，否则不建议作为交付主入口。

---

## 5. Compose 部署方式总览

| 文件 | 场景 | 数据存储 | 是否自带 PostgreSQL/Redis |
|---|---|---|---|
| `deploy/docker-compose.local.yml` | 推荐生产/迁移友好 | 本地目录 | 是 |
| `deploy/docker-compose.yml` | 常规生产 | Docker named volumes | 是 |
| `deploy/docker-compose.standalone.yml` | 外部 DB/Redis | named volume（仅应用数据） | 否 |
| `deploy/docker-compose.dev.yml` | 本地源码调试 | 本地目录 | 是 |

---

## 6. 标准部署流程（推荐：local 版本）

### 6.1 获取代码

```bash
git clone git@github.com:lauchiwa/sub2api.git
cd sub2api/deploy
```

如果目标机器没有 Git，也可以直接打包整个仓库上传。

### 6.2 准备环境变量

```bash
cp .env.example .env
```

然后编辑 `.env`，至少确认以下项目：
- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `TOTP_ENCRYPTION_KEY`
- `ADMIN_EMAIL`
- `ADMIN_PASSWORD`（可留空自动生成）
- `TZ`

建议使用安全随机值：

```bash
openssl rand -hex 32
```

可用于生成：
- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `TOTP_ENCRYPTION_KEY`

### 6.3 创建本地数据目录

```bash
mkdir -p data postgres_data redis_data
```

### 6.4 启动服务

```bash
docker compose -f docker-compose.local.yml up -d
```

### 6.5 查看状态与日志

```bash
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml logs -f sub2api
```

### 6.6 访问服务
默认访问地址：

```text
http://<服务器IP>:8080
```

如果未手动设置 `ADMIN_PASSWORD`，首次启动后请在日志中查找自动生成的管理员密码。

---

## 7. 命名卷版部署流程（docker-compose.yml）
适合希望把数据交给 Docker volume 管理、且不强调目录级迁移的场景。

### 7.1 启动

```bash
cd deploy
cp .env.example .env
# 编辑 .env

docker compose up -d
```

### 7.2 常用命令

```bash
docker compose ps
docker compose logs -f sub2api
docker compose restart sub2api
docker compose pull
docker compose up -d
```

### 7.3 清理数据

```bash
docker compose down -v
```

> `-v` 会删除 named volumes，执行前请确认已备份。

---

## 8. Standalone 部署流程（外部 PostgreSQL / Redis）
适合以下场景：
- PostgreSQL 使用云数据库；
- Redis 使用托管服务；
- 只希望容器内跑 Sub2API 本体。

### 8.1 关键要求
使用 `deploy/docker-compose.standalone.yml` 时，必须在 `.env` 中提供：
- `DATABASE_HOST`
- `DATABASE_PORT`
- `DATABASE_USER`
- `DATABASE_PASSWORD`
- `DATABASE_DBNAME`
- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_PASSWORD`（如果你的 Redis 开了密码）

### 8.2 启动

```bash
cd deploy
cp .env.example .env
# 编辑数据库/Redis 连接项

docker compose -f docker-compose.standalone.yml up -d
```

---

## 9. 本地源码调试部署（dev compose）
当你正在修改当前仓库代码，想直接从源码构建并验证，可使用：

```bash
cd deploy
cp .env.example .env
mkdir -p data postgres_data redis_data
docker compose -f docker-compose.dev.yml up --build -d
```

查看日志：

```bash
docker compose -f docker-compose.dev.yml logs -f sub2api
```

停止：

```bash
docker compose -f docker-compose.dev.yml down
```

---

## 10. 关键环境变量说明
以下变量是最值得运维优先检查的：

| 变量 | 是否关键 | 说明 |
|---|---|---|
| `POSTGRES_PASSWORD` | 必填 | PostgreSQL 超级关键密码 |
| `POSTGRES_USER` | 建议 | 默认 `sub2api` |
| `POSTGRES_DB` | 建议 | 默认 `sub2api` |
| `SERVER_PORT` | 可选 | 默认容器内 8080，对外映射也依赖它 |
| `BIND_HOST` | 可选 | 默认 `0.0.0.0`，如只允许本机反代访问可改 `127.0.0.1` |
| `ADMIN_EMAIL` | 建议 | 初始管理员邮箱 |
| `ADMIN_PASSWORD` | 建议 | 留空可自动生成 |
| `JWT_SECRET` | 强烈建议固定 | 不固定会导致容器重启后会话失效 |
| `TOTP_ENCRYPTION_KEY` | 强烈建议固定 | 不固定会导致 2FA 配置失效 |
| `TZ` | 建议 | 默认 `Asia/Shanghai` |
| `UPDATE_PROXY_URL` | 可选 | 在线更新/访问 GitHub 受限时可配置代理 |
| `DATABASE_MAX_OPEN_CONNS` 等 | 可调优 | 数据库连接池参数 |
| `REDIS_POOL_SIZE` 等 | 可调优 | Redis 连接池参数 |

### 10.1 Gemini OAuth 相关
如需启用 Gemini 账号能力，还可配置：
- `GEMINI_OAUTH_CLIENT_ID`
- `GEMINI_OAUTH_CLIENT_SECRET`
- `GEMINI_OAUTH_SCOPES`
- `GEMINI_QUOTA_POLICY`

如不填，项目会按其默认逻辑运行。

---

## 11. 自动初始化行为
Compose 文件默认都设置了：

```text
AUTO_SETUP=true
```

这意味着首次启动时系统会自动：
1. 连接 PostgreSQL 与 Redis；
2. 执行数据库 migration；
3. 初始化配置文件；
4. 创建管理员账户；
5. 在必要时自动生成部分缺省值。

所以一般不需要再单独跑 Setup Wizard。

---

## 12. 数据目录与迁移策略

### 12.1 local 版本目录
`docker-compose.local.yml` 默认数据目录：
- `deploy/data`
- `deploy/postgres_data`
- `deploy/redis_data`

### 12.2 迁移到新服务器
如果你用的是 local 版本，迁移最简单：

```bash
cd /path/to/sub2api
docker compose -f deploy/docker-compose.local.yml down

tar czf sub2api-deploy.tar.gz deploy/
scp sub2api-deploy.tar.gz user@new-server:/path/to/
```

新机器上：

```bash
tar xzf sub2api-deploy.tar.gz
cd deploy
docker compose -f docker-compose.local.yml up -d
```

---

## 13. 常用运维命令

### 13.1 查看容器状态

```bash
docker compose -f docker-compose.local.yml ps
```

### 13.2 查看主服务日志

```bash
docker compose -f docker-compose.local.yml logs -f sub2api
```

### 13.3 重启主服务

```bash
docker compose -f docker-compose.local.yml restart sub2api
```

### 13.4 更新镜像并重启

```bash
docker compose -f docker-compose.local.yml pull
docker compose -f docker-compose.local.yml up -d
```

### 13.5 停止所有服务

```bash
docker compose -f docker-compose.local.yml down
```

### 13.6 删除 local 版全部数据

```bash
docker compose -f docker-compose.local.yml down
rm -rf data postgres_data redis_data
```

> 该操作不可恢复，执行前务必确认备份。

---

## 14. 故障排查

### 14.1 页面打不开 / 8080 无法访问
检查：

```bash
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml logs --tail=200 sub2api
```

同时确认：
- 宿主机防火墙是否放行 8080；
- `BIND_HOST` 是否设置成了 `127.0.0.1`；
- 是否有 Nginx/Caddy/安全组拦截。

### 14.2 启动卡在数据库连接
优先检查：
- `POSTGRES_PASSWORD` 是否与数据库容器一致；
- PostgreSQL healthcheck 是否通过；
- 外部数据库场景下 `DATABASE_HOST` / `DATABASE_PORT` 是否正确。

### 14.3 Redis 连接失败
优先检查：
- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_PASSWORD`
- `REDIS_ENABLE_TLS`

### 14.4 重启后所有登录失效
通常是 `JWT_SECRET` 没有固定。

### 14.5 2FA 突然全部不可用
通常是 `TOTP_ENCRYPTION_KEY` 没有固定。

### 14.6 宿主机挂载目录权限异常
仓库已提供 `deploy/docker-entrypoint.sh`，会在启动时尝试修正 `/app/data` 所有权；如仍失败，请检查：
- 挂载目录是否只读；
- 宿主机文件系统权限是否允许容器进程修改；
- 是否存在 SELinux / AppArmor 限制。

---

## 15. 生产环境建议

1. **优先用反向代理暴露 80/443**，Sub2API 内部仍跑 8080。
2. **不要把 PostgreSQL / Redis 直接暴露公网**。
3. **固定 `JWT_SECRET` 与 `TOTP_ENCRYPTION_KEY`**。
4. **定期备份 `data`、数据库和 Redis 持久化数据**。
5. **如拉取镜像或依赖慢，及时切换国内加速源**。
6. **如果需要目录级迁移和可移交性，优先使用 `docker-compose.local.yml`**。

---

## 16. 对外发送说明（可直接转给运维）
可直接转发下面这段：

> 这边已经把 Sub2API 的 Docker 化部署资产整理好了，仓库内可直接使用。生产建议优先走 `deploy/docker-compose.local.yml`，因为数据会落在部署目录本地文件夹里，备份和迁移都最简单。部署时只需要进入 `deploy/`，复制 `.env.example` 为 `.env`，填好 `POSTGRES_PASSWORD`、`JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 等关键变量，创建 `data/postgres_data/redis_data` 目录后执行 `docker compose -f docker-compose.local.yml up -d` 即可。若数据库和 Redis 已外置，则改用 `docker-compose.standalone.yml`。详细步骤见仓库根目录 `DEPLOY_DOCKER.md`。
