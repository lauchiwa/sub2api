# Type Safety

> tsconfig 的严格开关、类型放在哪、运行时校验怎么做。

---

## 编译器配置

`frontend/tsconfig.json`（TypeScript `~5.6.0`）：

```jsonc
{
  "strict": true,
  "noUnusedLocals": true,          // ★ 留一个没用到的变量/import 就编译失败
  "noUnusedParameters": true,
  "noFallthroughCasesInSwitch": true,
  "isolatedModules": true,
  "moduleResolution": "bundler",
  "paths": { "@/*": ["./src/*"] },
  "types": ["vite/client"]
}
```

- `include` 只有 `src/**/*.{ts,tsx,vue}`，**`__tests__` 与 `*.spec.ts` 被 exclude**——
  测试文件不参与 `vue-tsc` 检查（由 vitest 自己跑）。
- 类型检查有两条入口，**都必须通过**：
  - `pnpm typecheck` → `vue-tsc --noEmit`
  - `pnpm build` → `vue-tsc -b && vite build`，且 `vite.config.ts` 里
    `vite-plugin-checker` 开了 `vueTsc: true`，**开发时保存即报错**。
- `isolatedModules: true` 的直接后果：**导出/导入类型必须写 `type` 修饰**：

  ```ts
  import type { ApiKey, PaginatedResponse } from '@/types'
  export type { LoginResponse } from './auth'
  ```

- 路径一律 `@/xxx`（`vite.config.ts` 和 `vitest.config.ts` 都配了同名 alias）。

---

## 类型放在哪

| 类型 | 位置 |
|------|------|
| 后端接口的请求/响应结构、领域实体 | `src/types/index.ts`（2270 行，按 `// ===== X Types =====` 分节） |
| 支付相关类型 | `src/types/payment.ts` |
| 全局声明（`window.__APP_CONFIG__` 等） | `src/types/global.d.ts` |
| 路由 meta | `src/router/meta.d.ts` |
| 组件专属 props/emits 类型 | 组件文件内 `interface Props` |
| 基础组件的公共类型（列定义等） | `src/components/common/types.ts` |
| feature 私有类型 | `src/features/<name>/types.ts` |
| API 模块内的窄类型（如某接口特有的 filter） | 直接写在 `src/api/<域>.ts` 的函数签名里 |

判定：**只被一个组件使用 → 就地定义；跨文件复用或对应后端结构 → `src/types/`。**
不要在多个组件里重复声明同一个后端结构。

通用泛型已在 `src/types/index.ts` 备好，直接复用：

```ts
export interface BasePaginationResponse<T> { items: T[]; total: number; page: number; page_size: number; pages: number }
export interface FetchOptions { signal?: AbortSignal }
export interface SelectOption { value: string | number | boolean | null; label: string; [key: string]: any }
```

字段名保持后端的 snake_case（`page_size`、`created_at`），**不做驼峰转换**——
全站一致，转换只会制造两套命名。

---

## 运行时校验

**没有引入 Zod / Yup 之类的 schema 库**，采用手写窄化 + 容错解析：

```ts
// src/i18n/index.ts —— 类型守卫
function isLocaleCode(value: string): value is LocaleCode {
  return value === 'en' || value === 'zh'
}

// src/stores/auth.ts —— 解析持久化 JSON 时逐字段校验，脏数据直接清掉
const parsed = JSON.parse(raw) as Partial<PendingAuthSessionSummary> | null
const provider = typeof parsed?.provider === 'string' ? parsed.provider.trim() : ''
if (!provider) { localStorage.removeItem(KEY); return null }
```

必须做运行时校验的三个边界：

1. **`localStorage` / `sessionStorage` 读出的 JSON** —— 可能是旧版本结构。
2. **`window.__APP_CONFIG__`** —— 由服务端注入的 HTML 内联脚本。
3. **URL query / route params** —— 用户可任意构造。

`api/client.ts` 里对响应体也做了 `typeof data === 'object' && data !== null` 的形状校验，
**目的是防止反代返回 HTML 错误页时崩掉错误处理**，新增分支要保留这个前提。

---

## 常用模式

```ts
// 字面量联合代替 enum（全项目不用 TS enum）
type ChangeType = 'up' | 'down' | 'neutral'
type LocaleCode = 'en' | 'zh'

// 组件 props
interface Props { title: string; value: number | string; icon?: Component }
const props = withDefaults(defineProps<Props>(), { ... })

// 泛型 composable
export function useTableLoader<T, P extends Record<string, any>>(options: TableLoaderOptions<T, P>)

// 未知错误先窄化再取字段
function extractApiErrorCode(err: unknown): string | undefined {
  if (!err || typeof err !== 'object') return undefined
  const e = err as ApiErrorLike
  return (e.reason ?? e.code) != null ? String(e.reason ?? e.code) : undefined
}
```

- **枚举用字面量联合**，运行时常量放 `src/constants/`（`account.ts`、`channel.ts`、
  `channelMonitor.ts`），二者配套定义。
- catch 到的错误标 `unknown`，用 `ApiErrorLike` 这类局部 interface 窄化后再取字段。

---

## 禁止 / 慎用

| 模式 | 说明 |
|------|------|
| `any` 泛滥 | ESLint 的 `@typescript-eslint/no-explicit-any` 被关掉了（历史遗留），**这不是许可**。新代码用 `unknown` + 窄化；只有对接 `SelectOption` 的额外属性、第三方库回调这类确实无法标注的地方才用 `any` |
| `as` 强转链（`x as unknown as Y`） | 仅限拦截器/错误处理这类已有先例的边界；业务代码用类型守卫 |
| `!` 非空断言 | 用可选链 + 默认值代替 |
| `@ts-ignore` | `ban-ts-comment` 虽已关闭，但新代码不要用；必须用时写 `@ts-expect-error` 并加原因注释 |
| TS `enum` | 与 `isolatedModules` 配合差，用字面量联合 |
| 在组件里重复声明后端结构 | 从 `@/types` import |
| 遗留未使用的 import / 变量 | `noUnusedLocals` 直接让 build 失败 |

---

## 常见错误

- **改完代码只跑 `pnpm dev` 没跑 `pnpm typecheck`** —— CI/构建阶段才爆。
- **`import { SomeType }` 忘了 `type` 关键字** —— `isolatedModules` 报错。
- **给后端字段做驼峰转换** —— 与 `src/types` 和 API 模块的 snake_case 约定冲突。
- **catch 里直接 `err.message`** —— `err` 是 `unknown`，用 `extractApiErrorMessage`。
- **用 `??` 覆盖了合法的 `0` / `''`** —— 数值/字符串默认值要显式判空。
