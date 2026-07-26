# Common Components

跨页面复用的基础组件。当前 38 个 `.vue`。

> **组件 API 以源码为准。** 每个组件的 props / emits / slots 直接看文件里的
> `defineProps<Props>()`、`defineEmits<...>()` 和模板中的 `<slot>`——
> 这里不再维护逐组件的属性清单（此前维护的版本已与代码脱节）。
>
> 编写规范见 [`.trellis/spec/frontend/component-guidelines.md`](../../../../.trellis/spec/frontend/component-guidelines.md)。

## 导入方式

`index.ts` 只导出了常用的一批（`DataTable`、`Pagination`、`BaseDialog`、`ConfirmDialog`、
`StatCard`、`Toast`、`LoadingSpinner`、`EmptyState`、`LocaleSwitcher`、`ExportProgressDialog`）
以及 `Column` 类型：

```typescript
import { DataTable, ConfirmDialog } from '@/components/common'
import type { Column } from '@/components/common'
```

不在 barrel 里的组件直接按路径导入：

```typescript
import GroupSelector from '@/components/common/GroupSelector.vue'
```

新增组件时**不要求**一定挂进 `index.ts`，但挂了就要两种导入方式都能用。

## 需要知道的几处结构关系

- **弹窗基于 `BaseDialog.vue`。** 它负责遮罩、Teleport、Escape / 点击外部关闭、焦点处理。
  全仓 55 个 `*Modal.vue` / `*Dialog.vue` 里 **50 个基于它**，
  `ConfirmDialog.vue` 和 `ExportProgressDialog.vue` 是它之上的封装。
  **新弹窗一律走 `BaseDialog`，不要另写一套遮罩逻辑。**
  现存例外只有 4 个 TOTP / 2FA 弹窗（`TotpLoginModal`、`TotpStepUpDialog`、
  `TotpSetupModal`、`TotpDisableDialog`），它们自带遮罩——属于存量，不是可效仿的写法。
- **Toast 是全局单例机制**：`Toast.vue` 在布局里挂载一次，
  内容由 `useAppStore()` 驱动。业务代码不直接操作它，而是调
  `appStore.showSuccess / showError / showWarning / showInfo`
  （详见 [`.trellis/spec/frontend/state-management.md`](../../../../.trellis/spec/frontend/state-management.md)）。
- **表单控件**：`Input.vue`、`Select.vue`、`TextArea.vue`、`Toggle.vue`、`SearchInput.vue`
  是统一入口，不要在业务组件里裸写 `<input class="...">`。
- **`types.ts`** 放跨组件共享的类型（如 `DataTable` 的 `Column`），
  只被单个组件使用的类型就地定义在该组件内。

## 什么该放进 common/

判定标准：**被两个以上不同领域（admin / user / auth）使用，且不含业务语义**。
只在某一领域内复用的组件放 `components/admin/`、`components/user/` 等对应目录。

## 约定

- `<script setup lang="ts">`，Tailwind 样式，必须带 `dark:` 变体。
- 用户可见文案走 i18n（`useI18n()`），不硬编码。
- `v-html` 必须先消毒（`utils/sanitize.ts` 的 `sanitizeSvg`，或 `marked` + `DOMPurify`）。
