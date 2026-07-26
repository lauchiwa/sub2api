# API Layer

> `src/api/` 的组织方式、axios 单例的拦截器行为，以及调用方必须知道的错误形状。

---

## 单一 axios 实例

所有请求都走 `src/api/client.ts` 导出的 `apiClient`：

```ts
export const apiClient: AxiosInstance = axios.create({
  baseURL: getAPIBaseURL(),   // 来自 src/api/url.ts
  withCredentials: true,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' }
})
```

**禁止在业务代码里直接 `axios.get(...)` 或新建 axios 实例**——会绕过鉴权注入、
信封拆包和 401 刷新逻辑。唯一的例外在 `client.ts` 内部：token 刷新用裸 `axios.post`
以避免拦截器递归，且**显式带 `timeout: 30000`**（裸 axios 默认无限等待，挂起会让
`isRefreshing` 永远为 true，所有排队请求死锁）。

---

## 请求拦截器做了什么

| 行为 | 说明 |
|------|------|
| `Authorization: Bearer <token>` | 从 `localStorage.auth_token` 取 |
| `Accept-Language` | 取 `getLocale()`，后端据此本地化错误文案 |
| GET 请求追加 `timezone` 参数 | `Intl.DateTimeFormat().resolvedOptions().timeZone`，后端用于默认时间范围 |
| 管理端/用户端 UI 标记头 | `shouldMarkAdminUIRequest` / `shouldMarkUserUIRequest`（`src/api/adminUIRequest.ts`）决定是否加 `ADMIN_UI_REQUEST_HEADER` / `USER_UI_REQUEST_HEADER` |

新增需要区分「来自 UI」与「来自 API 客户端」的接口时，改 `adminUIRequest.ts` 的匹配规则，
不要在调用点手写这两个头。

---

## 响应拦截器：信封拆包

后端 `/api/v1/**` 统一返回 `{ code, message, reason, metadata, data }`：

```ts
const apiResponse = response.data as ApiResponse<unknown>
if (apiResponse && typeof apiResponse === 'object' && 'code' in apiResponse) {
  if (apiResponse.code === 0) {
    response.data = apiResponse.data          // ★ 成功：拆包，只留 data
  } else {
    return Promise.reject({ status: response.status, code, message, reason, metadata })
  }
}
```

**`code === 0` 才是成功**（不是 200）。这与后端 `internal/pkg/response/response.go` 一一对应，
后端 `internal/server/api_contract_test.go` 锁定了这个契约——**任何一侧改动都要同步另一侧**。

对调用方的含义：`const { data } = await apiClient.get<ApiKey>('/keys/1')` 里的 `data`
**已经是 `ApiKey`**，不需要再 `.data.data`。

---

## API 模块写法

按域一个文件，导出具名 async 函数，参数与返回值都显式标类型：

```ts
// src/api/keys.ts
import { apiClient } from './client'
import type { ApiKey, PaginatedResponse } from '@/types'

export async function list(
  page: number = 1,
  pageSize: number = 10,
  filters?: { search?: string; status?: string; sort_by?: string; sort_order?: 'asc' | 'desc' },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<ApiKey>> {
  const { data } = await apiClient.get<PaginatedResponse<ApiKey>>('/keys', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  return data
}
```

约定：

- **列表接口必须支持 `options?.signal`**，透传给 axios。表格切页/搜索时靠它取消旧请求
  （见 [`useTableLoader`](./composable-guidelines.md#usetableloader)）。
- 查询参数用后端的 snake_case（`page_size`、`sort_by`、`group_id`），不要在这里转驼峰。
- 每个导出函数写 JSDoc（现有文件全部有），说明参数含义与默认值。
- 在 `src/api/index.ts` 里统一再导出；管理端接口经 `src/api/admin/index.ts` 汇总为 `adminAPI`。

模块划分与后端路由分组对齐：`keys` / `usage` / `user` / `redeem` / `payment` / `groups` /
`channels` / `subscriptions` / `totp` / `announcements` / `batchImage` / `channelMonitor` / `setup`，
管理端另有 31 个 `admin/*.ts`。

---

## 401 与 token 刷新

`client.ts` 维护一个模块级刷新队列：

```ts
let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []
```

流程：

1. 收到 401 且 `!originalRequest._retry` 且不是 `/auth/login|register|refresh` 且本地有
   `refresh_token` → 进入刷新分支。
2. 已有刷新在进行 → 当前请求挂进 `refreshSubscribers` 等待，拿到新 token 后重放。
3. 刷新成功 → 更新 `auth_token` / `refresh_token` / `token_expires_at`，重放原请求。
4. 刷新失败 → 清空四个 localStorage key，`sessionStorage.auth_expired = '1'`，跳 `/login`，
   并以 `{ status: 401, code: 'TOKEN_REFRESH_FAILED' }` reject。

**改这段逻辑要极其小心**：`_retry` 标记防止无限重试，`onTokenRefreshed('')` 用于在失败时
唤醒所有等待者，漏掉任何一步都会造成页面永久 loading。相关测试在 `src/api/__tests__/`。

---

## 拦截器里的特殊分支

除通用逻辑外，响应拦截器还处理三类特例，新增同类需求时在同一位置扩展：

| 条件 | 行为 |
|------|------|
| `ERR_CANCELED` / `axios.isCancel` | **原样 reject 原始取消错误**，不包装成网络错误——调用方据此静默忽略 |
| 404 且 `message === 'Ops monitoring is disabled'` | 写 `localStorage.ops_monitoring_enabled_cached='false'`，派发 `ops-monitoring-disabled` 事件，若在 `/admin/ops` 下跳走 |
| 423 且 `code === 'ADMIN_COMPLIANCE_ACK_REQUIRED'` | 派发 `admin-compliance-required` 事件，由 `useAdminComplianceStore` 接管 |

拦截器会把 `data` 做 `typeof data === 'object'` 校验后再取字段，
**目的是防止反代返回 HTML 错误页时崩掉错误处理**——新增分支时保持这个前提。

---

## 错误形状与消费方式

拦截器 reject 的**不是 AxiosError，而是普通对象**：

```ts
{ status, code, reason, error, message, metadata }
// 网络错误：{ status: 0, message: 'Network error. Please check your connection.' }
```

调用方统一用 `src/utils/apiError.ts` 提取，不要自己解 `err.response.data`：

```ts
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'

// 直接拿可展示消息
appStore.showError(extractApiErrorMessage(err, t('common.error')))

// 按后端 reason 查 i18n，并用 metadata 做占位符插值
appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
```

`extractApiErrorCode` **优先取字符串 `reason`**（如 `PAYMENT_PROVIDER_MISCONFIGURED`）
而非数字 HTTP `code`，因为只有 `reason` 的粒度足以驱动 i18n 查表。
后端新增错误分支时，配套在 `i18n` 对应 namespace 下加 `reason` 同名 key。

配套工具：`src/utils/authError.ts`、`errorCategory.ts`、`errorBadges.ts`。

---

## 常见错误

- **直接用 `axios` 而不是 `apiClient`** —— 丢鉴权头、丢信封拆包、丢刷新逻辑。
- **以为响应里还有 `.data.data`** —— 成功响应已被拆包。
- **列表接口不接 `signal`** —— 快速切页/输入搜索时旧响应覆盖新数据。
- **在组件里 `catch (err) { err.response.data.message }`** —— 拦截器 reject 的是普通对象，
  没有 `response`；用 `extractApiErrorMessage`。
- **把取消错误当失败弹 toast** —— 判断 `code === 'ERR_CANCELED'` 或用
  `useTableLoader` 内置的 `isAbortError`。
- **改后端信封字段却没改 `client.ts`** —— 全站请求静默失败。
