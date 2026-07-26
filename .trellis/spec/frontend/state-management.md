# State Management

> Pinia setup store 的写法、状态归属判定、以及 localStorage 持久化约定。

---

## 状态分四类

| 类别 | 载体 | 例 |
|------|------|-----|
| 组件局部状态 | `ref` / `reactive` in `<script setup>` | 弹窗开关、当前编辑的行 |
| 可复用的实例状态 | composable（每个调用方一份） | `useTableLoader` 的分页与列表 |
| 跨组件共享状态 | Pinia store（全局单例） | 登录用户、站点公开配置、toast 队列 |
| URL 状态 | vue-router 的 `params` / `query` | 页面 id、跳转来源 |

**默认从局部开始**。只有当「两个不相邻组件必须看到同一份数据」时才提升到 store，
当前只有 8 个 store，保持这个克制程度。

---

## Store 一律 setup 风格

`src/stores/` 下 8 个 store 全部是 setup 语法，**不用 options 语法**：

```ts
// src/stores/app.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAppStore = defineStore('app', () => {
  // ==================== State ====================
  const sidebarCollapsed = ref<boolean>(false)
  const toasts = ref<Toast[]>([])

  // ==================== Computed ====================
  const hasActiveToasts = computed(() => toasts.value.length > 0)

  // ==================== Actions ====================
  function toggleSidebar(): void { sidebarCollapsed.value = !sidebarCollapsed.value }

  // ==================== Return Store API ====================
  return { sidebarCollapsed, toasts, hasActiveToasts, toggleSidebar }
})
```

约定：

- `defineStore('<id>', () => {...})`，id 与文件名一致。
- 文件内用 `// ==================== X ====================` 分节，顺序为
  State → Computed → Actions → Return。这是现有全部 store 的统一格式。
- **末尾显式 return 需要暴露的东西**；不想暴露的内部变量（如 `app.ts` 里的
  `publicSettingsRequest`、`toastIdCounter`）用普通 `let` 声明并不 return。
- 每个 store 在 `src/stores/index.ts` 里再导出一次。
- 不用 `pinia-plugin-persistedstate`，持久化手写（见下）。

---

## 现有 store 与职责

| Store | 职责 |
|-------|------|
| `useAuthStore` | 登录态、token 生命周期、用户信息、待完成的第三方登录会话 |
| `useAppStore` | 侧边栏、全局 loading 计数、toast 队列、站点公开配置缓存、版本信息 |
| `useAdminSettingsStore` | 管理端设置与自定义菜单 |
| `useAdminComplianceStore` | 管理端合规确认（响应 `admin-compliance-required` 事件） |
| `useSubscriptionStore` / `usePaymentStore` | 订阅与支付流程状态 |
| `useAnnouncementStore` | 公告拉取与已读状态 |
| `useOnboardingStore` | 新手引导进度 |

`src/stores/README.md` 是目录级索引：store 一览表 + 导入约定 + localStorage key 表。
它**不再罗列逐个 action**（旧版本罗列过，结果只覆盖 2 个 store 且已脱节）。
新增 store 时往那张表加一行；state/action 以代码为准。

---

## 服务端状态怎么缓存

项目**没有引入 TanStack Query 之类的服务端状态库**，缓存逻辑手写在 store 里。
标准形态见 `app.ts` 的 `fetchPublicSettings`：

```ts
let publicSettingsRequest: Promise<PublicSettings | null> | null = null

function fetchPublicSettings(force = false): Promise<PublicSettings | null> {
  // 1. 进行中的请求永远优先，保证所有调用方观察到同一次刷新结果
  if (publicSettingsRequest) return publicSettingsRequest
  // 2. 服务端注入的 window.__APP_CONFIG__（消除首屏闪烁）
  if (!publicSettingsLoaded.value && !force && window.__APP_CONFIG__) { ... }
  // 3. 已有缓存且未强制刷新
  if (publicSettingsLoaded.value && !force) return Promise.resolve({ ...cached })
  // 4. 真正发请求，finally 里清空 in-flight 句柄
}
```

要点：

- **in-flight 去重**：用模块级 `Promise` 句柄，避免多个组件同时挂载时打出并发请求。
- **`force` 参数**：所有带缓存的 fetch 都提供 `force = false` 形参用于强制刷新。
- **提供 `clearXxxCache()`**：数据变更后由调用方主动失效。
- `window.__APP_CONFIG__` 由后端在 HTML 里注入（开发模式由 `vite.config.ts` 的
  `injectPublicSettings` 插件模拟），用于首屏免请求，**不要绕过它直接发请求**。

---

## loading 与 toast 统一走 appStore

```ts
const appStore = useAppStore()

await appStore.withLoading(() => api.save(payload))                 // 只管 loading
const r = await appStore.withLoadingAndError(() => api.save(p), msg) // loading + 失败 toast，返回 null

appStore.showSuccess(t('common.saved'))   // 默认 3000ms
appStore.showError(msg)                   // 默认 5000ms
appStore.showWarning(msg)                 // 默认 4000ms
appStore.showInfo(msg)                    // 默认 3000ms
```

`setLoading` 内部是**计数器**（`loadingCount`），支持嵌套调用，
所以务必成对调用或直接用 `withLoading`，手写 `setLoading(true)` 后忘记关会让全局
loading 永久亮着。

`<Toast />` 只在 `App.vue` / 布局里挂一次，组件不要自己渲染 toast 容器。

---

## localStorage / sessionStorage 约定

- **key 必须提到文件顶部作为常量**，不要散在调用点：

  ```ts
  const AUTH_TOKEN_KEY = 'auth_token'
  const TOKEN_EXPIRES_AT_KEY = 'token_expires_at'   // 存过期时间戳而非有效期
  ```

- 认证相关的四个 key 由 `stores/auth.ts` 与 `api/client.ts` **共同读写**：
  `auth_token`、`refresh_token`、`auth_user`、`token_expires_at`，
  外加 `pending_auth_session`（第三方登录未完成时的中间态）。
  **改任何一个都要同时看这两个文件**，否则刷新逻辑会与 store 失配。
- `sessionStorage.auth_expired = '1'` 是「会话过期」一次性信号，登录页消费后清除。
- 表格偏好（隐藏列、排序、页大小）走 `src/utils/tablePreferences.ts` /
  `usePersistedPageSize`，key 带版本后缀（`*-version`）便于结构变更时失效。
- 其他持久化：`sub2api_locale`（语言）、`sub2api:ip-geo-cache:v1`（IP 归属缓存）、
  `ops_monitoring_enabled_cached`、`affiliate_referral_code`。
- **读写必须容错**：`localStorage` 在隐私模式/配额满时会抛异常，
  照 `client.ts` 的写法用 `try { ... } catch { /* ignore */ }` 包住。
- 解析持久化的 JSON 要做字段校验并在失败时删除脏数据
  （参考 `auth.ts` 的 `getPersistedPendingAuthSession`）。

---

## 组件里怎么用

**主流写法是持有 store 实例、按属性访问**（全仓库仅 1 处用了 `storeToRefs`）：

```ts
const authStore = useAuthStore()
const appStore = useAppStore()

// 模板与 computed 里直接用 authStore.user / authStore.isAuthenticated
authStore.logout()
```

确实需要解构成独立 ref 时，**必须用 `storeToRefs`**（参考
`components/common/AnnouncementBell.vue`）：

```ts
import { storeToRefs } from 'pinia'
const { announcements, loading } = storeToRefs(announcementStore)
```

直接 `const { user } = useAuthStore()` 会丢响应式。action 可以直接解构。

---

## 常见错误

- **直接解构 store 的 state** —— 丢响应式，界面不更新。
- **把只有一个页面用的状态放进 store** —— 该用组件局部状态或 composable。
- **手写 `setLoading(true)` 后异常路径没关** —— 用 `withLoading`。
- **绕过 `appStore` 自己弹提示** —— toast 样式和层级不一致。
- **localStorage key 硬编码在多处** —— 改名时漏改，登录态莫名丢失。
- **改了 auth 相关 key 只改 store 没改 `api/client.ts`** —— 401 刷新链路断掉。
- **在 store 里 import 组件、或直接写 axios/HTTP 细节** —— store 只依赖 `@/api` 的封装函数。
