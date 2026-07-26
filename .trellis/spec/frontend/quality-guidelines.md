# Quality Guidelines

> ESLint 规则、测试要求、CI 关卡与提交前检查清单。

---

## 只用 pnpm

仓库只有 `frontend/pnpm-lock.yaml`，CI（`.github/workflows/backend-ci.yml` 的 `frontend` job、
`release.yml`、`security-scan.yml`）全部用 `pnpm install --frozen-lockfile`。

- **禁止 `npm install` / `yarn`**，会生成冲突的 lockfile 并让 `--frozen-lockfile` 失败。
- 改了 `package.json` **必须一起提交 `pnpm-lock.yaml`**。
- 新增依赖前先确认是否已有等价能力（`@vueuse/core` 覆盖了大量常见需求）。

---

## 命令

```bash
cd frontend
pnpm install --frozen-lockfile

pnpm dev              # vite，默认 3000，代理 /api /v1 /setup 到 VITE_DEV_PROXY_TARGET
pnpm lint:check       # eslint（不改文件）—— CI 用这个
pnpm lint             # eslint --fix
pnpm typecheck        # vue-tsc --noEmit
pnpm test:run         # vitest run（全量）
pnpm test:coverage    # vitest run --coverage
pnpm build            # vue-tsc -b && vite build → ../backend/internal/web/dist
```

仓库根目录：

```bash
make test-frontend            # lint:check + typecheck + test-frontend-critical
make test-frontend-critical   # 只跑关键路径测试子集
make build-frontend           # pnpm --dir frontend run build
```

---

## CI 实际跑什么

`backend-ci.yml` 的 `frontend` job 只执行 **`make test-frontend`**，即：

1. `pnpm lint:check`
2. `pnpm typecheck`
3. `vitest run` **仅关键路径子集**（`Makefile` 的 `FRONTEND_CRITICAL_VITEST`）：

```
src/views/auth/__tests__/LinuxDoCallbackView.spec.ts
src/views/auth/__tests__/WechatCallbackView.spec.ts
src/views/user/__tests__/PaymentView.spec.ts
src/views/user/__tests__/PaymentResultView.spec.ts
src/components/user/profile/__tests__/ProfileInfoCard.spec.ts
src/views/admin/__tests__/SettingsView.spec.ts
```

**注意含义**：CI 不跑全量 189 个测试文件。改动非关键路径时，
**本地必须自己跑 `pnpm test:run`**，否则回归到合并后才暴露。
若新写的测试守护的是登录 / 支付 / 设置这三条主链路，考虑把它加进 `FRONTEND_CRITICAL_VITEST`。

另有 `security-scan.yml`（每周）跑 `pnpm audit --prod --audit-level=high`，
例外白名单在 `tools/check_pnpm_audit_exceptions.py`。

---

## ESLint

`frontend/.eslintrc.cjs`：`eslint:recommended` + `plugin:vue/vue3-essential` +
`plugin:@typescript-eslint/recommended`，parser 为 `vue-eslint-parser`（内嵌 `@typescript-eslint/parser`）。

已关闭的规则及其含义：

| 关闭的规则 | 说明 |
|-----------|------|
| `@typescript-eslint/no-explicit-any` | **不是许可**——新代码仍用 `unknown` + 窄化，见 [Type Safety](./type-safety.md#禁止--慎用) |
| `@typescript-eslint/ban-types` / `ban-ts-comment` | 存量兼容；新代码不要写 `@ts-ignore` |
| `vue/multi-word-component-names` | 允许 `Input.vue` / `Select.vue` 这类单词组件名 |
| `vue/no-use-v-if-with-v-for` | 存量兼容；**新代码不要在同一元素上写 v-if + v-for** |
| `no-unused-vars`（原生） | 由 `@typescript-eslint/no-unused-vars` 接管，`^_` 前缀豁免 |

未使用变量是 **warn**，但 `tsconfig` 的 `noUnusedLocals` 会让它变成**编译失败**——
以 `pnpm typecheck` 为准。

代码风格（缩进、引号、分号）**没有 Prettier 强制**。跟随所在文件的既有风格：
`.ts` 用 2 空格缩进、单引号、无分号结尾（新文件按此写）。

---

## 测试

- 框架：Vitest 2 + `@vue/test-utils` 2 + jsdom，`globals: true`。
- 位置：`__tests__/<被测名>.spec.ts`，与被测文件同级目录。当前 189 个测试文件。
- 全局 setup 在 `src/__tests__/setup.ts`，已 mock：内存版 `localStorage`、
  `requestIdleCallback` / `cancelIdleCallback`、`matchMedia`（默认 `matches: true`，
  即桌面视口）、`IntersectionObserver`、`ResizeObserver`；全局超时 10s。
  **需要新的浏览器 API mock 时加在这里**，不要每个测试各 mock 一份。
- `vitest.config.ts` 配了 80% 覆盖率阈值（statements / branches / functions / lines），
  仅在 `pnpm test:coverage` 时生效，CI 未强制。

**该测什么**：

- 必测：OAuth 回调解析、支付状态流转、鉴权守卫、`api/client.ts` 的信封与 401 刷新、
  从 View 抽出的纯逻辑 `.ts`（`groupsModelsList.ts` 等）、i18n key 完整性。
- 应测：新 composable 的主路径与异常路径；新增基础组件的 props/emits 行为。
- 不必测：纯展示组件的模板细节、第三方库行为。

**反模式**：把被测逻辑整体删掉后测试仍然通过（同义反复测试），
判定方法见 [`../guides/index.md`](../guides/index.md)。

---

## 必须遵守的模式

- 所有请求走 `@/api` 的封装，**不直接用 axios**（见 [API Layer](./api-layer.md)）。
- 用户可见文案一律 i18n，**en / zh 同时补齐**（见 [i18n Guidelines](./i18n-guidelines.md)）。
- 组件带 `dark:` 变体（见 [Component Guidelines](./component-guidelines.md#样式tailwind-优先)）。
- 分页表格用 `useTableLoader`；表单提交用 `useForm`；提示用 `appStore.showXxx`。
- 副作用（定时器、监听、AbortController）在 `onUnmounted` 里清理。
- 引用用 `@/` 别名，不跨目录写 `../../..`。
- `localStorage` 读写用 try/catch 包裹，key 提为顶部常量。

## 禁止的模式

| 禁止 | 原因 | 替代 |
|------|------|------|
| `npm install` / `yarn` | 生成冲突 lockfile | `pnpm` |
| 业务代码里 `axios.get(...)` | 绕过鉴权、信封拆包、401 刷新 | `@/api` 的模块函数 |
| 硬编码用户可见文案 | 切语言残留 | `t('...')` |
| 未消毒的 `v-html` | XSS | `sanitizeSvg` 或 `marked` + `DOMPurify` |
| Options API / `defineComponent` | 与全仓 276 个 `<script setup>` 不一致 | `<script setup lang="ts">` |
| `pinia` options 语法 store | 与现有 8 个 setup store 不一致 | setup 语法 |
| 手写分页三件套 | 缺请求取消，旧响应覆盖新数据 | `useTableLoader` |
| 遗留未使用的 import | `noUnusedLocals` 让构建失败 | 删掉 |
| 直接解构 store state | 丢响应式 | `store.x` 或 `storeToRefs` |
| 改 `backend/internal/web/dist/` | 构建产物 | 改 `frontend/src` 后 `pnpm build` |

---

## 提交前检查清单

```bash
cd frontend
pnpm lint:check
pnpm typecheck
pnpm test:run          # 注意 CI 只跑关键子集，本地要跑全量
```

- [ ] `pnpm lint:check` 无新增问题
- [ ] `pnpm typecheck` 通过
- [ ] `pnpm test:run` 全量通过（尤其 `src/i18n/__tests__/`）
- [ ] 新文案 en / zh 都补齐了
- [ ] 新组件在暗色模式下可读
- [ ] 改了 `package.json` → `pnpm-lock.yaml` 已同步提交
- [ ] 改了响应信封 / 错误字段 → 后端 `internal/pkg/response` 与
      `internal/server/api_contract_test.go` 已同步

---

## Code Review 关注点

1. 有没有绕过 `apiClient` 直接发请求。
2. 列表接口是否透传 `signal`，取消错误是否被误当失败。
3. 文案是否 i18n 化、en/zh 是否对称。
4. 暗色模式是否覆盖。
5. 定时器 / 监听 / 控制器是否在 `onUnmounted` 清理。
6. 新状态该放局部、composable 还是 store，是否放错层。
7. `v-html` 是否消毒。
8. 是否引入了新依赖，能否用 `@vueuse/core` 或现有工具替代。

AI 交叉评审结果需按 [`../guides/index.md`](../guides/index.md) 的验证规则复核。
