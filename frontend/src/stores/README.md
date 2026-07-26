# Pinia Stores

全部 8 个 store 一律使用 **setup 语法**（`defineStore('name', () => { ... })`），
不用 options 语法。

> **store 的 state / actions 以源码为准。** 这里不再维护逐个 action 的清单
> （此前维护的版本只覆盖了 2 个 store，且已与代码脱节）。
>
> 状态分层判定、缓存模式、localStorage 约定见
> [`.trellis/spec/frontend/state-management.md`](../../../.trellis/spec/frontend/state-management.md)。

## Store 一览

| 文件 | 名称 | 职责 |
|------|------|------|
| `auth.ts` | `useAuthStore` | 登录 / 注册 / 登出、2FA、token 持久化与自动刷新、当前用户 |
| `app.ts` | `useAppStore` | 全局 UI：侧栏折叠、全局 loading 计数、toast 队列 |
| `adminSettings.ts` | `useAdminSettingsStore` | 管理端设置的读取与缓存 |
| `subscriptions.ts` | `useSubscriptionStore` | 订阅套餐与当前订阅状态 |
| `payment.ts` | `usePaymentStore` | 支付下单与状态流转 |
| `announcements.ts` | `useAnnouncementStore` | 公告列表与已读状态 |
| `onboarding.ts` | `useOnboardingStore` | 新手引导步骤进度 |
| `adminCompliance.ts` | `useAdminComplianceStore` | 管理端合规 / 审计相关状态 |

统一从 barrel 导入（`index.ts` 同时重导出了 `User`、`Toast` 等常用类型）：

```typescript
import { useAuthStore, useAppStore } from '@/stores'
```

## 使用约定

- **持有 store 实例、通过 `store.x` 访问**，这是全仓的主流写法。
  确实需要解构响应式字段时才用 `storeToRefs`（目前只有 `AnnouncementBell.vue` 这么做）。
- **store 只依赖 `@/api` 的封装函数**，不直接写 axios / HTTP 细节，也不 import 组件。
- 提示统一走 `appStore.showSuccess / showError / showWarning / showInfo`；
  异步操作可用 `appStore.withLoading` / `withLoadingAndError` 包裹
  （`loading` 由内部 `loadingCount` 计数驱动，支持并发调用）。
- **不是所有共享状态都该进 store**：只在一个页面树内使用的状态留在组件或 composable 里。

## 持久化

`auth.ts` 与 `api/client.ts` **共用同一批 `localStorage` key**，改动必须两边同步：

| key | 用途 |
|-----|------|
| `auth_token` | access token |
| `refresh_token` | refresh token |
| `auth_user` | 当前用户快照（JSON） |
| `token_expires_at` | 过期**时间戳**（不是有效期秒数） |
| `pending_auth_session` | 待完成的第三方登录会话（JSON） |

- `api/client.ts` 的 401 刷新流程会直接读写前 4 个 key；
  登出 / 刷新失败时四个一起清除。
- 读取持久化 JSON 必须 try/catch 并逐字段校验，脏数据直接清掉——
  旧版本结构可能残留在用户浏览器里。
- `app.ts` 不做持久化（UI 状态刷新即重置）。
