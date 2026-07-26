# Reverse Proxy

> Nginx / Caddy / CDN 前置时的配置基线、转发 IP 信任模型与流式请求的超时约定。
> 源文档：`deploy/EDGE_SECURITY.md`、`deploy/Caddyfile`、`README_CN.md` §Nginx 反向代理注意事项。

---

## 前提：应用侧已有的边界

反代配置必须与应用默认值配套，不要在边缘再叠一层更严的限制把正常请求掐掉：

| 配置项 | 默认值 | 含义 |
|--------|--------|------|
| `server.max_header_bytes` | `65536`（64 KiB） | HTTP/1 请求头上限，同时约束 HTTP/2 header list |
| `server.read_header_timeout` | `10`（秒） | 只限制读完请求头，**不限制处理与响应流** |
| `server.max_request_body_size` | `268435456`（256 MiB） | 全局请求体安全网 |
| `gateway.max_body_size` | `268435456` | 多模态 / Gemini / 图像 / 视频 / 批量图像端点 |
| `gateway.text_max_body_size` | `33554432`（32 MiB） | 纯文本端点 `/embeddings`、`/alpha/search` |
| `server.h2c` | 50 流/连接、2 MiB 连接上传窗口、512 KiB 流上传窗口 | h2c 参数 |
| 无效凭据限流 | 每可信客户端 IP（IPv6 按 `/64`）60 秒内 120 次失败 → 封禁 60 秒 | **单实例安全网**，多实例仍需 LB/CDN/WAF |

**绝对不要给应用配置响应 `WriteTimeout`**，也不要加应用级全局请求信号量——
理由见 [index](./index.md#三条最容易踩的硬规则)。

---

## Nginx 基线

`http` 块里定义共享 zone（速率值是保守起点，**按实测流量调整**，不是通用容量目标）：

```nginx
limit_conn_zone $binary_remote_addr zone=sub2api_conn:20m;
limit_req_zone  $binary_remote_addr zone=sub2api_auth:20m rate=5r/s;
limit_req_zone  $binary_remote_addr zone=sub2api_api:40m rate=30r/s;
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

# ★ 必须：Nginx 默认丢弃带下划线的请求头（如 session_id），
#   会导致多账号环境下的粘性会话失效。
underscores_in_headers on;

server {
    listen 443 ssl http2;
    server_name api.example.com;

    client_header_timeout 10s;
    client_max_body_size 256m;
    large_client_header_buffers 4 16k;
    limit_conn sub2api_conn 40;

    # 认证端点单独限流，防撞库
    location ~ ^/(auth|api/auth)/ {
        limit_req zone=sub2api_auth burst=10 nodelay;
        proxy_pass http://127.0.0.1:8080;
    }

    # 纯文本端点收紧到 32m，与 gateway.text_max_body_size 对齐
    location ~ ^/(v1/)?(embeddings|alpha/search)$ {
        client_max_body_size 32m;
        limit_req zone=sub2api_api burst=60 nodelay;
        proxy_pass http://127.0.0.1:8080;
    }

    location / {
        limit_req zone=sub2api_api burst=60 nodelay;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;   # ★ 覆写，不是 $proxy_add_x_forwarded_for
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_buffering off;                              # ★ SSE 必须关缓冲
        proxy_request_buffering off;
        proxy_read_timeout 1800s;                         # ★ 长生成
        proxy_send_timeout 1800s;
        proxy_pass http://127.0.0.1:8080;
    }
}
```

**四条不可省略的点**：

1. `underscores_in_headers on;`（`http` 块级）—— 粘性会话依赖 `session_id` 头。
2. `proxy_buffering off; proxy_request_buffering off;` —— 否则 SSE 被攒包，客户端看不到增量输出。
3. `proxy_read_timeout` / `proxy_send_timeout` 放到 30 分钟量级 —— 长生成不被中途切断。
4. `X-Forwarded-For $remote_addr` **覆写**而非 `$proxy_add_x_forwarded_for` 追加。

**禁止**在未把 Nginx 的 real-IP 处理限制到明确可信代理 CIDR 的前提下，
直接使用进来的 `$http_x_forwarded_for` 值。

---

## Caddy 基线

仓库自带 `deploy/Caddyfile`，是**客户端直连 Caddy** 的基线：

- 全局：`max_header_size 64KB`、`timeouts { read_header 10s; idle 2m }`
- TLS：仅 `tls1.2 tls1.3` + 指定加密套件
- `request_body { max_size 256MB }`
- `reverse_proxy localhost:8080`，健康检查 `/health`（30s 间隔），
  故障转移 `fail_duration 30s` / `max_fails 3` / `unhealthy_status 500 502 503 504`
- **转发头从真实 TCP 对端生成**：`header_up X-Real-IP {remote_host}`、
  `header_up X-Forwarded-For {remote_host}`

> Caddy core 没有通用限流器。需要限流时用 CDN/WAF、受支持的 rate-limit 模块，
> 或主机防火墙。

### CDN 后面的 Caddy（必须改）

**不要原样沿用 `{remote_host}` 那两行**——所有客户端都会被归到 CDN 出口地址，
拒绝聚合与无效凭据限流会打到无关用户身上。

正确做法（先把源站防火墙限制到 CDN 出口 CIDR，再用 Caddy 解析出的 `{client_ip}`）：

```caddyfile
{
	servers {
		trusted_proxies static 192.0.2.0/24 2001:db8:1234::/48
		trusted_proxies_strict
		client_ip_headers CF-Connecting-IP X-Forwarded-For
	}
}

api.example.com {
	reverse_proxy 127.0.0.1:8080 {
		header_up X-Real-IP {client_ip}
		header_up X-Forwarded-For {client_ip}
	}
}
```

示例里的网段要替换成 CDN 官方公布、自动维护的出口范围。
`CF-Connecting-IP` 在这里安全**仅仅因为**源站直连已被封锁且 Caddy 只信任那些 TCP 对端。
同时把应用的 `server.trusted_proxies` 设成 Caddy 的地址/私网段。

---

## 客户端 IP 信任模型（最容易配错的部分）

应用有两种模式，由 `security.trust_forwarded_ip_for_api_key_acl` 切换：

### 兼容模式（默认 `true`）

- 原始转发头接管客户端 IP 解析，用于日志与安全敏感路径。
- 自定义头列表 `security.forwarded_client_ip_headers` **按配置顺序**优先检查，
  之后才回退到内置的 `CF-Connecting-IP` → `X-Real-IP` → `X-Forwarded-For`。
- 头名大小写不敏感，加载时归一化去重，**上限 16 个合法 HTTP 字段名**。
- 头值必须是 IP 字面量；支持逗号分隔，非法项跳过，**公网地址优先于私网回退地址**。
- 可用 YAML 配置，也可用逗号分隔的环境变量 `SECURITY_FORWARDED_CLIENT_IP_HEADERS`；
  **显式设为空的环境变量会清空 YAML 值**。管理端安全设置也可改，**运行时生效无需重启**。
- 一次请求会把「开关 + 头列表」一起快照，不会混用新旧配置。
- ⚠️ **兼容模式接受转发头时不校验直连对端**（包括自定义头）。
  启用期间**必须**从网络层保护源站不被直接访问。

### 高安全模式（`false`）

- 自定义头被完全忽略，**Gin 的 `server.trusted_proxies` 链是唯一权威**。
- `trusted_proxies` 只填**直接连接 Sub2API** 的精确 CIDR/IP：

  ```yaml
  server:
    trusted_proxies:
      - 127.0.0.1/32
      - ::1/128
  ```

- 显式空列表 `[]` = 不信任任何转发客户端 IP。

### 升级行为

首次升级到该模式时，历史值 `false` 只有在**未显式配置过 `server.trusted_proxies`**
的情况下才会被改成 `true`；显式配置过代理策略的实例保持安全模式。
新装会在数据库初始化时持久化自定义头列表；老装从 YAML 回填缺失值，
并用隐藏迁移标记防止之后管理员的修改被覆盖。
读设置失败或持久化的头列表格式非法时，进程**fail closed** 到「可信代理模式 + 无自定义头」。

---

## CDN / WAF 层职责

- 连接数限制、请求头/体大小限制、Bot 挑战、按 IP/ASN 的速率限制——都在流量到达源站前完成。
- 源站入站**只放行 CDN 出口 CIDR 或私有 LB**，应用端口不要暴露到公网。
- 代理必须**覆写**每一个受信任的客户端 IP 头，而不是把不可信的客户端值追加上去。

---

## DDoS 边界（明确不做什么）

应用层检查只能在连接进入 Go 之后削弱放大效应，**无法吸收**：
体积型攻击、TLS 洪水、带宽饱和、大规模分布式源。
这些需要上游网络容量、CDN/WAF 过滤、云厂商防火墙规则和源站隔离。

拒绝风暴期间**避免高基数指标与每请求写库的安全日志**——
参见 [`../backend/logging-guidelines.md`](../backend/logging-guidelines.md) 的
`logger.OpsSystemLogSkipField`。

---

## 检查清单

- [ ] `underscores_in_headers on;`（Nginx）
- [ ] `proxy_buffering off` + `proxy_request_buffering off`
- [ ] `proxy_read_timeout` / `proxy_send_timeout` ≥ 最长生成时间
- [ ] 未给应用配置 `WriteTimeout`
- [ ] `client_max_body_size` 与 `gateway.max_body_size` 对齐（256m），文本端点 32m
- [ ] 认证路径有独立的更严限流
- [ ] 转发 IP 头是**覆写**而非追加
- [ ] CDN 场景：源站防火墙只放行 CDN 出口 CIDR
- [ ] `server.trusted_proxies` 只包含直连代理地址
- [ ] `/health` 可达（Docker healthcheck 与 Caddy 健康检查都用它）
