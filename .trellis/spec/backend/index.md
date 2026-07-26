# Backend Development Guidelines

> sub2api 后端（Go）的架构规范与编码约定。所有规则均从当前代码库实际实现中提取。

---

## 技术栈（实际版本）

| 项 | 值 | 来源 |
|----|----|------|
| Go | **1.26.5**（CI 强校验 `go version \| grep -q 'go1.26.5'`） | `backend/go.mod`、`.github/workflows/backend-ci.yml` |
| Module | `github.com/Wei-Shaw/sub2api` | `backend/go.mod` |
| HTTP 框架 | `github.com/gin-gonic/gin` v1.9.1 | `backend/go.mod` |
| ORM | `entgo.io/ent` v0.14.5（代码生成） | `backend/ent/` |
| DI | `github.com/google/wire` v0.7.0（编译期注入） | `backend/cmd/server/wire.go` |
| 配置 | `github.com/spf13/viper` v1.18.2 | `backend/internal/config/config.go` |
| 日志 | `go.uber.org/zap` v1.24.0 + `lumberjack.v2` 轮转 | `backend/internal/pkg/logger/` |
| 数据库 | PostgreSQL（生产）/ `modernc.org/sqlite`（无 CGO 场景） | `backend/internal/repository/ent.go` |
| 缓存 | `github.com/redis/go-redis/v9` v9.17.2 | `backend/internal/repository/*_cache.go` |
| 定时任务 | `github.com/robfig/cron/v3` | `backend/internal/service/` |
| Lint | golangci-lint **v2**（`version: "2"` schema） | `backend/.golangci.yml` |

升级依赖前先确认 `.golangci.yml`、`Dockerfile`（`GOLANG_IMAGE=golang:1.26.5-alpine`）、CI 的 Go 版本断言三处同步。

---

## Guidelines Index

| Guide | 内容 |
|-------|------|
| [Architecture](./architecture.md) | 分层边界、依赖倒置、Wire DI、depguard 硬约束 |
| [Directory Structure](./directory-structure.md) | 目录布局、文件命名、新功能落点 |
| [Database Guidelines](./database-guidelines.md) | Ent schema、mixins、软删除、事务、SQL 迁移 |
| [Error Handling](./error-handling.md) | `ApplicationError`、响应信封、错误传播 |
| [Logging Guidelines](./logging-guidelines.md) | zap 全局 logger、component 命名、日志脱敏 |
| [Quality Guidelines](./quality-guidelines.md) | golangci-lint 规则、gofmt rewrite、禁止模式 |
| [Testing Guidelines](./testing-guidelines.md) | `unit`/`integration`/`e2e` build tag 体系、testutil |

跨层改动请同时读 [`../guides/cross-layer-thinking-guide.md`](../guides/cross-layer-thinking-guide.md)。
部署 / 反代相关请读 [`../deployment/index.md`](../deployment/index.md)。

---

## 三条最容易踩的硬规则

1. **`service` 与 `handler` 禁止 import `repository` / `redis` / `gorm`**。由 `.golangci.yml` 的 depguard 强制，违反直接 CI 失败。详见 [Architecture](./architecture.md)。
2. **改了 `ent/schema/*.go` 必须 `go generate ./ent` 并提交生成代码**。`ent/` 下 13 万行生成代码是版本库的一部分。
3. **给 interface 加方法后，所有测试 stub/mock 必须补全**，否则编译失败。搜索方式见 [Testing Guidelines](./testing-guidelines.md)。

---

## 语言约定

代码注释与本规范采用 **中文叙述 + 英文技术术语** 的混合风格，与现有代码一致
（例：`// 统一使用 ent 的事务：保证用户与允许分组的更新原子化`，见 `internal/repository/user_repo.go`）。
标识符、包名、错误 reason、日志 component 一律英文。
