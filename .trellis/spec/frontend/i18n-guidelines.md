# i18n Guidelines

> vue-i18n 的懒加载机制、语言包目录结构、key 命名与已有的校验测试。

---

## 配置要点

`src/i18n/index.ts`：

```ts
export const i18n = createI18n({
  legacy: false,             // Composition 模式，组件里用 useI18n()
  locale: getDefaultLocale(),
  fallbackLocale: 'en',
  messages: {},              // ★ 初始为空，语言包按需加载
  warnHtmlMessage: false     // 引导步骤用富文本（driver.js 支持 HTML）
})
```

配套约束：

- 支持的语言只有 **`en` / `zh`**，默认 `en`；首次访问按 `navigator.language` 推断。
- 用户选择持久化在 `localStorage.sub2api_locale`。
- `vite.config.ts` 把 `vue-i18n` 别名指向 **runtime 版本**
  （`vue-i18n.runtime.esm-bundler.js`）并开启 `__INTLIFY_JIT_COMPILATION__`，
  目的是**避免 CSP 下的 `unsafe-eval`**。
  → **不要改成完整版 `vue-i18n`，也不要在运行时用字符串拼 message 模板。**
- `vitest.config.ts` 里配了同样的别名，测试环境保持一致。

---

## 懒加载与语言切换

```ts
await loadLocaleMessages('zh')   // 动态 import('./locales/zh')，带 loadedLocales 去重
await initI18n()                 // main.ts 启动时加载当前语言并设置 <html lang>
await setLocale('zh')            // 切换：加载 → 设 locale → 写 localStorage → 更新 <html lang> → 刷新页签标题
```

`setLocale` 末尾会**重新解析当前路由的文档标题**（动态 import `@/router/title`、
`@/stores/app`、`@/stores/auth`、`@/stores/adminSettings`），保证切语言时页签标题跟随。
这里用动态 import 是为了避免 i18n 与 store/router 的循环依赖——**新增依赖时保持动态 import**。

切语言的 UI 统一用 `components/common/LocaleSwitcher.vue`。

---

## 语言包目录结构

```
src/i18n/locales/
├── en/
│   ├── index.ts        # 聚合导出（default export）
│   ├── common.ts       # 通用词：确认/取消/错误/加载中…
│   ├── dashboard.ts  landing.ts  batchImage.ts  misc.ts
│   └── admin/
│       ├── index.ts
│       ├── accounts.ts  audit.ts  channels.ts  ops.ts
│       ├── overview.ts  promptAudit.ts  resources.ts  settings.ts
└── zh/                 # 结构与 en 完全一一对应
```

规则：

- **en 与 zh 的文件树、key 树必须完全对称。** 新增一个 key 就要在两边都加。
- 文案量大的领域拆独立文件（`admin/settings.ts` 之类），不要往 `common.ts` 里堆。
- 新增文件后记得在同目录 `index.ts` 里挂上，否则整棵子树都取不到。

---

## key 命名

- 路径式点号命名，与领域/页面对齐：
  `common.confirm`、`auth.createAccount`、`admin.settings.payment.field_certSerial`、
  `payment.errors.PAYMENT_PROVIDER_MISCONFIGURED`。
- 段名小驼峰；**后端错误 `reason` 作为 key 时保留原样的 UPPER_SNAKE**，
  因为 `extractI18nErrorMessage` 直接用 `${namespace}.${reason}` 查表
  （见 [API Layer](./api-layer.md#错误形状与消费方式)）。
- 插值用具名占位符 `{count}` / `{name}`，不要用位置参数。
- **禁止跨域复用 key**（比如在 admin 里引用 user 的 key），后续删改会互相牵连。

---

## 组件里的用法

```ts
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
```

- 245 个 `.vue` 用了 `useI18n()`，模板里 `{{ t('xxx') }}`，属性上 `:title="t('xxx')"`。
- **必须在 setup 同步阶段调用 `useI18n()`**，不能在 `await` 之后。
- store / 工具函数等非组件上下文里用全局实例：

  ```ts
  import { i18n } from '@/i18n'
  i18n.global.t('common.unknownError')     // 见 stores/app.ts
  ```

---

## 已有的校验测试（会拦住你）

`src/i18n/__tests__/` 下的测试专门守护语言包完整性：

| 测试 | 守护什么 |
|------|---------|
| `localesNoKeyCollision.spec.ts` | key 不重复、不冲突 |
| `localesMessageCompile.spec.ts` | 所有 message 能被 `@intlify/message-compiler` 编译（占位符语法正确） |
| `opsLocaleKeys.spec.ts`、`riskControlLocales.spec.ts`、`ipGeoLocales.spec.ts`、`usageServiceTierLocales.spec.ts`、`wsModeLocaleDesc.spec.ts`、`openaiFastPolicyLocales.spec.ts` | 特定业务域的 key 在 en/zh 两侧齐全 |

**加了新文案要跑一遍 `pnpm test:run src/i18n`**。这些测试失败最常见的原因就是只补了中文。

---

## 常见错误

- **只加 zh 不加 en**（或反之）—— i18n 测试失败；线上表现为回退成 key 字符串。
- **模板里硬编码文案** —— 切语言残留。
- **在 `await` 之后调 `useI18n()`** —— 拿不到组件实例。
- **把 vue-i18n 别名改回完整版** —— 引入 `unsafe-eval`，违反 CSP。
- **占位符写错**（`{ count }` 带空格、用 `%s`）—— `localesMessageCompile` 直接报错。
- **新语言包文件没挂进 `index.ts`** —— 整个子树取不到值。
- **用后端返回的中文直接展示** —— 后端已按 `Accept-Language` 本地化
  （`client.ts` 会带上该头），但错误分支应优先用 `reason` 查前端 i18n。
