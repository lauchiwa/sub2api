# Directory Structure

> `frontend/src/` 的实际布局、新代码该放哪、以及命名约定。

---

## 顶层布局

```
frontend/
├── src/
│   ├── main.ts                # 应用入口：Pinia / router / i18n 装配
│   ├── App.vue
│   ├── api/           (66)    # 后端接口封装，按域拆文件
│   │   ├── client.ts          # ★ axios 单例 + 拦截器
│   │   ├── url.ts             # baseURL / 网关 URL 推导
│   │   ├── adminUIRequest.ts  # 管理端/用户端请求标记头
│   │   ├── index.ts           # 统一出口
│   │   └── admin/     (31)    # 管理端接口，与 admin 视图对应
│   ├── stores/        (12)    # Pinia store（setup 风格）
│   ├── composables/   (34)    # use* 组合式函数
│   ├── components/   (271)    # 组件，按领域分子目录
│   ├── views/        (145)    # 路由页面，按角色分子目录
│   ├── features/      (15)    # 垂直切片模块（当前仅 prompt-audit）
│   ├── router/         (8)    # 路由表、meta 类型、标题解析、setup 重定向
│   ├── i18n/          (39)    # 语言包（en / zh）与懒加载逻辑
│   ├── types/          (3)    # 全局类型：index.ts / payment.ts / global.d.ts
│   ├── utils/         (54)    # 纯函数工具，无 Vue 依赖
│   ├── constants/      (3)    # account / channel / channelMonitor 枚举常量
│   ├── assets/ styles/ style.css
│   └── __tests__/             # 全局 setup.ts 与跨模块集成测试
├── vite.config.ts             # 构建产物输出到 ../backend/internal/web/dist
├── vitest.config.ts
├── tailwind.config.js
├── tsconfig.json / tsconfig.node.json
└── .eslintrc.cjs
```

括号内为该目录下 `.vue` / `.ts` 文件数量，用于判断规模量级。

---

## components/ 分层

```
components/
├── common/      # 跨领域基础组件：DataTable、Pagination、BaseDialog、ConfirmDialog、
│                # Toast、Select、Input、TextArea、Toggle、StatCard、EmptyState、
│                # Skeleton、LoadingSpinner、SearchInput、DateRangePicker …
│                # 有 index.ts 桶导出与 types.ts
├── icons/       # Icon.vue 统一入口 + 各平台图标
├── layout/      # 侧边栏、顶栏、页面骨架
├── charts/      # chart.js 封装
├── admin/       # 管理端业务组件
├── user/        # 用户端业务组件
├── account/ channels/ keys/ auth/ payment/ Guide/
└── TurnstileWidget.vue
```

- **`common/` 的判定标准**：不含业务语义、任意页面可复用。带业务字段（账号、渠道、密钥）的
  组件放对应领域目录，不要塞进 `common/`。
- `common/README.md` 记录导入方式（`index.ts` barrel 只导出常用的一批）、
  弹窗基于 `BaseDialog.vue` 的层次关系、以及「什么该进 common/」的判定。
  它**不罗列逐组件 props**——组件 API 以 `defineProps` 为准。

---

## views/ 分层

```
views/
├── HomeView.vue  KeyUsageView.vue  NotFoundView.vue
├── auth/     # 登录、注册、找回密码、各 OAuth 回调（LinuxDo / 微信 / 钉钉 / OIDC / 通用）
├── user/     # 用户端：Dashboard、Keys、Usage、Subscriptions、Payment*、Redeem、Profile…
├── admin/    # 管理端：Users、Accounts、Channels、Groups、Settings、Usage、Ops…
│             # 含 ops/ orders/ settings/ affiliates/ 子目录
├── public/   # 免登录页（LegalDocumentView）
└── setup/    # 首次部署引导（SetupWizardView）
```

`views/admin/` 下还有若干**纯逻辑 `.ts` 同级文件**（`groupsModelsList.ts`、
`codexFingerprintSignals.ts`、`apiKeyGroupFilterOptions.ts` …）：
当某个 View 的选项计算/映射逻辑变复杂时，抽到同目录 `.ts` 便于单测，而不是继续堆在 SFC 里。
`views/user/` 同理（`paymentUx.ts`、`paymentWechatResume.ts`）。
**这是本项目处理超大页面的既定手法**，优先沿用。

---

## features/ —— 垂直切片

`src/features/prompt-audit/` 是目前唯一的切片模块，内部自带 `api.ts` / `types.ts` /
`viewModel.ts` / `components/` / `__tests__/`：

```
features/prompt-audit/
├── PromptAuditView.vue     # 页面入口
├── viewModel.ts            # 页面状态与业务逻辑（脱离 SFC，可单测）
├── api.ts  types.ts
├── components/             # 仅本特性使用的组件
└── __tests__/
```

**什么时候新建 feature 目录**：一个功能同时拥有独立的 API、类型、多个专属组件和可测试的
view model，且不与其他页面共享。否则按 `views/` + `components/<领域>/` 的常规方式组织。

---

## 新代码落点判定

| 要加的东西 | 放哪 |
|-----------|------|
| 调后端某个新接口 | `src/api/<域>.ts`（管理端接口进 `src/api/admin/<域>.ts`），并在 `index.ts` 导出 |
| 一个新页面 | `src/views/<角色>/XxxView.vue` + `src/router/index.ts` 加懒加载路由 |
| 页面里复杂的选项/映射计算 | 同目录 `xxx.ts`，单独写测试 |
| 跨页面复用的 UI | `src/components/common/`，并加进 `common/index.ts` |
| 某领域专用 UI | `src/components/<领域>/` |
| 有响应式状态的可复用逻辑 | `src/composables/useXxx.ts` |
| 纯函数（格式化、校验、解析） | `src/utils/xxx.ts`（**不要 import vue**） |
| 跨页面共享的服务端/会话状态 | `src/stores/xxx.ts` + `stores/index.ts` 导出 |
| 后端返回结构的类型 | `src/types/index.ts` |
| 枚举/常量 | `src/constants/<域>.ts` |
| 文案 | `src/i18n/locales/en/**` **和** `src/i18n/locales/zh/**` |

---

## 命名约定

| 对象 | 规则 | 例 |
|------|------|-----|
| 组件文件 | PascalCase `.vue` | `DataTable.vue`、`GroupSelector.vue` |
| 页面组件 | PascalCase + `View` 后缀 | `UsersView.vue`、`PaymentResultView.vue` |
| composable | `useXxx.ts`，导出同名函数 | `useTableLoader.ts` → `useTableLoader()` |
| store | 小驼峰文件名，导出 `useXxxStore` | `adminSettings.ts` → `useAdminSettingsStore` |
| API 模块 | 小驼峰，复数领域名 | `keys.ts`、`channels.ts`、`admin/riskControl.ts` |
| 工具 | 小驼峰，动词或名词短语 | `maskApiKey.ts`、`formatters.ts` |
| 测试 | `__tests__/<被测名>.spec.ts` | `src/api/__tests__/client.spec.ts` |

- 目录名：`common`、`admin`、`user` 等小写；`Guide/` 是历史遗留的大写目录，不要新增同类。
- 引用一律用 `@/` 别名（`tsconfig.json` 与 `vite.config.ts` 都配了），
  **禁止 `../../..` 式跨目录相对路径**（同目录 / 子目录相对引用可以）。

---

## 不要手动改的目录

- `backend/internal/web/dist/` —— `pnpm build` 的产物，由 Vite 覆盖写入。
- `node_modules/`、`pnpm-lock.yaml`（只能由 pnpm 命令更新）。
