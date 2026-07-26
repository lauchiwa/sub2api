# Deployment Guidelines

> 边缘反代、运行时配置与构建产物的规范。对应仓库的 `deploy/`、根 `Dockerfile` 与
> `backend/internal/config/`。

---

## 部署形态

Sub2API 是**单二进制 + PostgreSQL + Redis**：前端构建产物内嵌进 Go 二进制，
对外只暴露一个 HTTP 端口（默认 `8080`），前面通常再放一层反向代理。

```
客户端 (Claude Code / Codex CLI / Gemini CLI / 浏览器)
        │  TLS
        ▼
  反向代理（Caddy / Nginx / CDN+WAF）      ← 限流、连接数、TLS、转发头改写
        │  http://127.0.0.1:8080
        ▼
  sub2api 单进程（Gin + 内嵌前端 dist）
        │
        ├── PostgreSQL 18
        └── Redis 8
```

官方给出的编排见 `deploy/docker-compose.yml`（应用 + `postgres:18-alpine` +
`redis:8-alpine`，两个数据服务**不对宿主机暴露端口**）。
另有 `docker-compose.dev.yml` / `.local.yml` / `.standalone.yml` 三个变体。

---

## 三条最容易踩的硬规则

1. **不要给应用加 `WriteTimeout`，也不要在应用层加全局请求信号量。**
   网关承载长时间 SSE / WebSocket，写超时会掐断正常生成；一个 SSE 请求可能合法占用
   信号量数分钟。连接数与未认证请求的限制**属于边缘层**。
   （出处：`deploy/EDGE_SECURITY.md`）
2. **Nginx 必须开 `underscores_in_headers on;`**，否则 `session_id` 这类带下划线的头
   会被丢弃，多账号粘性会话失效。
3. **CDN/多层代理下必须同时做三件事**：防火墙只放行 CDN 出口 CIDR、代理层**覆写**
   （而非追加）客户端 IP 头、应用侧 `server.trusted_proxies` 只填直连的代理地址。
   任一缺失都会把所有用户归并到同一个 IP，拖垮限流与风控聚合。

---

## Guidelines Index

| Guide | 内容 |
|-------|------|
| [Reverse Proxy](./reverse-proxy.md) | Nginx / Caddy 基线配置、可信代理与转发 IP、SSE/WS 超时 |
| [Runtime Config](./runtime-config.md) | viper 配置加载顺序、环境变量命名、Docker 构建与镜像约定 |

相关规范：后端服务器配置项见 [`../backend/index.md`](../backend/index.md)，
前端构建产物落点见 [`../frontend/index.md`](../frontend/index.md)。

仓库内的权威文档（本规范由它们沉淀而来，冲突时以源文档为准）：

- `deploy/EDGE_SECURITY.md` —— 边缘与 HTTP ingress 安全（Nginx / Caddy / CDN 基线）
- `deploy/DOCKER.md`、`deploy/README.md` —— Docker 部署说明
- `README_CN.md` / `README.md` 的「Nginx 反向代理注意事项」
- `deploy/config.example.yaml` —— 全量配置项与逐项中英注释
