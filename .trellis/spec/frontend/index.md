# Frontend Development Guidelines

> `frontend/` 的技术栈、目录约定与编码规范。基于仓库实际代码沉淀，不是通用最佳实践。

---

## 技术栈与版本

| 领域 | 选型 | 版本（`frontend/package.json`） |
|------|------|------|
| 框架 | Vue | `^3.4.0`（**全部用 `<script setup lang="ts">`**） |
| 语言 | TypeScript | `~5.6.0`（`strict: true`） |
| 构建 | Vite | `^5.0.10` |
| 包管理 | **pnpm**（仓库只有 `pnpm-lock.yaml`） | — |
| 路由 | vue-router | `^4.2.5`（全部懒加载） |
| 状态 | Pinia | `^2.1.7`（**全部 setup 风格**） |
| HTTP | axios | `^1.18.0`（单例 + 拦截器） |
| i18n | vue-i18n | `^9.14.5`（Composition 模式 + 懒加载语言包） |
| 样式 | TailwindCSS | `^3.4.0`（`darkMode: 'class'`） |
| 图表 | chart.js + vue-chartjs | `^4.4.1` / `^5.3.0` |
| 工具集 | @vueuse/core | `^10.7.0` |
| 测试 | Vitest + @vue/test-utils + jsdom | `^2.1.9` / `^2.4.6` / `^24.1.3` |
| Lint | ESLint 8 + eslint-plugin-vue + @typescript-eslint | `^8.57.0` / `^9.25.0` / `^7.18.0` |
| 类型检查 | vue-tsc | `^2.2.0` |

规模参考：279 个 `.vue`（276 个 `<script setup lang="ts">`）、189 个测试文件、`src/types/index.ts` 2270 行。

---

## 与后端的两条硬耦合

1. **响应信封**：`src/api/client.ts` 的响应拦截器以 `code === 0` 判定成功并拆包 `data`。
   后端 `internal/pkg/response` 改动必须同步这里（见 [API Layer](./api-layer.md)）。
2. **构建产物内嵌进 Go 二进制**：`vite.config.ts` 的 `build.outDir` 是
   `../backend/internal/web/dist`，后端用 `//go:build embed` + `//go:embed all:dist` 打包。
   `pnpm build` 会**直接覆盖后端目录**，不是独立产物。

---

## Guidelines Index

| Guide | 内容 |
|-------|------|
| [Directory Structure](./directory-structure.md) | `src/` 布局、新代码落点、命名约定 |
| [API Layer](./api-layer.md) | axios 单例、信封拆包、401 刷新队列、按域拆分的 API 模块 |
| [Component Guidelines](./component-guidelines.md) | `<script setup>` 结构、props/emits、Tailwind 与暗色模式 |
| [Composable Guidelines](./composable-guidelines.md) | `use*` 组合式函数、表格加载/表单/OAuth 三类模式 |
| [State Management](./state-management.md) | Pinia setup store、localStorage key、局部 vs 全局状态 |
| [i18n Guidelines](./i18n-guidelines.md) | 语言包懒加载、模块拆分、key 命名与校验测试 |
| [Type Safety](./type-safety.md) | strict 配置、类型组织、`@/` 别名、禁止模式 |
| [Quality Guidelines](./quality-guidelines.md) | ESLint 规则、测试要求、提交前检查 |

后端规范见 [`../backend/index.md`](../backend/index.md)，部署与反代规范见
[`../deployment/index.md`](../deployment/index.md)。

---

## 三条最容易踩的硬规则

1. **改了 `package.json` 必须提交 `pnpm-lock.yaml`**，并且只用 pnpm（`npm install` 会生成
   冲突的 lockfile）。
2. **`tsconfig.json` 开了 `noUnusedLocals` + `noUnusedParameters`**，留一个没用到的
   import 就会让 `pnpm typecheck` 和 `vite build`（`vite-plugin-checker` 内联 vue-tsc）失败。
3. **新文案一律进 `src/i18n/locales/{en,zh}/`，en 与 zh 必须同时补齐**，
   `src/i18n/__tests__/` 下有多个 key 校验测试会直接失败。

---

## 语言约定

规范正文用中文，技术名词、代码标识符、路径保留英文——与本仓库代码注释的实际风格一致
（`vite.config.ts`、`client.ts`、`useTableLoader.ts` 均为中英混排）。
代码内注释沿用所在文件的既有语言，不要为统一风格批量改写。
