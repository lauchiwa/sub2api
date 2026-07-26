# Composable Guidelines

> `src/composables/` 的组合式函数：什么时候写、怎么写、现有的三类模式。

> 本项目是 Vue 3，没有 React hooks。Trellis 模板里的 `hook-guidelines.md` 已删除，
> 相关约定全部在本文件。

---

## composable vs util vs store

| 放哪 | 判据 | 例 |
|------|------|-----|
| `src/utils/` | **纯函数**，不 import vue，无响应式状态 | `formatters.ts`、`maskApiKey.ts`、`apiError.ts` |
| `src/composables/` | 有响应式状态或生命周期，但**每个使用者一份独立实例** | `useTableLoader`、`useForm`、`useClipboard` |
| `src/stores/` | 需要**跨组件共享同一份**状态 | `useAuthStore`、`useAppStore` |

判断口诀：两个组件同时用它，希望各自独立 → composable；希望看到同一份数据 → store。

---

## 基本写法

```ts
// src/composables/useForm.ts
import { ref } from 'vue'
import { useAppStore } from '@/stores/app'

interface UseFormOptions<T> {
  form: T
  submitFn: (data: T) => Promise<void>
  successMsg?: string
  errorMsg?: string
}

/**
 * 统一表单提交逻辑
 * 管理加载状态、错误捕获及通知
 */
export function useForm<T>(options: UseFormOptions<T>) {
  const loading = ref(false)
  const appStore = useAppStore()

  const submit = async () => {
    if (loading.value) return          // 防重复提交
    loading.value = true
    try {
      await options.submitFn(options.form)
      if (options.successMsg) appStore.showSuccess(options.successMsg)
    } catch (error) {
      appStore.showError(options.errorMsg ?? extractApiErrorMessage(error))
      throw error                      // ★ 继续抛出，让组件做局部处理
    } finally {
      loading.value = false
    }
  }

  return { loading, submit }
}
```

约定：

- 文件名 = 导出函数名 = `useXxx`，一个文件一个主 composable，**具名导出**（不用 default）。
- 参数多于两个时收进一个 `options` 对象，并为它声明 `interface UseXxxOptions`。
- 返回一个**扁平对象**，把 `ref` 直接暴露出去（调用方自己 `.value`）；
  不要返回 `reactive` 包裹的聚合对象。
- 顶部写中文 JSDoc 说明职责，与现有文件一致。
- **不吞异常**：需要 toast 的地方 toast，但仍要 `throw`，让调用方能做字段级校验展示。

---

## 三类现成模式（优先复用，别重写）

### useTableLoader

`src/composables/useTableLoader.ts` —— 所有分页表格的标准加载器，统一处理
**分页 + 筛选 + 搜索防抖 + 请求取消**：

```ts
const { items, loading, params, pagination, load, search } = useTableLoader<ApiKey, Filters>({
  fetchFn: keysAPI.list,        // (page, pageSize, params, options) => Promise<BasePaginationResponse<T>>
  initialParams: { status: '' },
  debounceMs: 300
})
```

关键点：

- 内部维护 `AbortController`，**每次 `load()` 先 abort 上一次**，防止旧响应覆盖新数据。
  所以传入的 `fetchFn` 必须接受 `options?: { signal }` 并透传给 axios。
- 用 `isAbortError()` 识别取消（`AbortError` / `ERR_CANCELED` / `CanceledError`）并静默忽略，
  其他错误照常抛出。
- 默认页大小来自 `usePersistedPageSize`（localStorage 持久化）。
- 搜索防抖用 `@vueuse/core` 的 `useDebounceFn`。

**新表格页面一律用它**，不要自己写 `page/pageSize/total` 三件套。
配套还有 `useTableSelection`（多选）、`usePersistedPageSize`、`useKeyedDebouncedSearch`。

### OAuth 授权流

`useAccountOAuth` / `useGeminiOAuth` / `useOpenAIOAuth` / `useGrokOAuth` /
`useAntigravityOAuth` —— 各平台的账号授权流程（弹窗、轮询、回调处理）形状一致。
**接入新平台时照 `useAccountOAuth.ts` 的结构复制**，不要另起一套交互模式。

### 表单与交互工具

`useForm`（提交 + loading + toast）、`useClipboard`、`useAutoRefresh`、
`useNavigationLoading`、`useRoutePrefetch`、`useSwipeSelect`、`useStepUp`（二次验证）、
`useOnboardingTour`（driver.js 引导）。

---

## 生命周期与清理

**任何注册了副作用的 composable 必须自己清理**——调用方不该记得去 stop：

```ts
import { onUnmounted } from 'vue'

let abortController: AbortController | null = null
const timer = setInterval(tick, 5000)

onUnmounted(() => {
  abortController?.abort()
  clearInterval(timer)
})
```

需要清理的东西：`setInterval` / `setTimeout`、`addEventListener`、`AbortController`、
WebSocket / EventSource、`watch` 的 stop handle（在 setup 作用域内自动清理，
在异步回调里创建的则要手动 stop）。

优先用 `@vueuse/core` 的封装（`useEventListener`、`useIntervalFn`、`useDebounceFn`），
它们自带 scope 清理。

---

## 调用位置限制

composable 内部若使用 `inject` / 生命周期钩子 / `useI18n()` / `useRouter()`，
**必须在组件 setup 的同步阶段调用**：

```ts
// ✅
const { t } = useI18n()
async function submit() { appStore.showError(t('common.error')) }

// ❌ await 之后拿不到当前实例
async function submit() {
  await api.save()
  const { t } = useI18n()
}
```

Pinia store（`useXxxStore()`）在 Pinia 安装后可以在任意位置调用，
但**仍建议在 setup 顶部取一次**存成常量，与现有代码一致。

---

## 测试

composable 的测试放 `src/composables/__tests__/<name>.spec.ts`，
纯逻辑的直接调用函数断言返回的 ref；依赖组件上下文的用 `@vue/test-utils` 挂一个
最小宿主组件。参见 [Quality Guidelines](./quality-guidelines.md#测试)。

---

## 常见错误

- **该用 store 却写了 composable** —— 两个组件各拿到一份状态，数据对不上。
- **忘记 `onUnmounted` 清理定时器/监听** —— 路由切走后仍在轮询，最终打爆后端。
- **`fetchFn` 不透传 `signal`** —— `useTableLoader` 的取消机制失效。
- **把取消错误当失败** —— 快速切页时满屏 error toast；用 `isAbortError` 过滤。
- **在 `await` 之后调用 `useI18n()` / `useRouter()`** —— 拿不到实例，运行时报错。
- **composable 里吞掉异常只 toast** —— 调用方无法做字段级错误展示；要 `throw`。
- **把纯函数写进 composables** —— 没有响应式状态的东西属于 `src/utils/`。
