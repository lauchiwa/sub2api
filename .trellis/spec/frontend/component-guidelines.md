# Component Guidelines

> SFC 的标准结构、props/emits 写法、Tailwind 与暗色模式约定。

---

## 一律 `<script setup lang="ts">`

279 个 `.vue` 里 276 个用 `<script setup lang="ts">`。**新组件没有例外**：
不写 Options API、不写 `defineComponent`、不用 JSX。

标准结构（`src/components/common/StatCard.vue`）：

```vue
<template>
  <div class="stat-card">
    <p class="stat-label truncate">{{ title }}</p>
    <p class="stat-value" :title="String(formattedValue)">{{ formattedValue }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'

type ChangeType = 'up' | 'down' | 'neutral'

interface Props {
  title: string
  value: number | string
  icon?: Component
  changeType?: ChangeType
  formatValue?: (value: number | string) => string
}

const props = withDefaults(defineProps<Props>(), {
  changeType: 'neutral'
})

const formattedValue = computed(() => {
  if (props.formatValue) return props.formatValue(props.value)
  return typeof props.value === 'number' ? props.value.toLocaleString() : props.value
})
</script>
```

块顺序固定为 **`<template>` → `<script setup>` → `<style>`**（`<style>` 通常不需要，见下）。

`<script setup>` 内部顺序：`import` → 类型定义 → `defineProps` / `defineEmits` →
store / composable 调用 → `ref` / `reactive` → `computed` → `watch` → 函数 →
生命周期钩子。

---

## Props

- **类型化声明**：`defineProps<Props>()`（193 个文件）配 `interface Props`，
  不写运行时对象形式。
- 有默认值时用 `withDefaults(defineProps<Props>(), { ... })`（59 个文件）。
- 可选 prop 用 `?:`，不要用 `| undefined` 加 `required: false` 的混合写法。
- prop 名在 `<script>` 里 camelCase，在模板上 kebab-case（`:change-type="'up'"`）。
- 复杂对象类型从 `@/types` import，不要在组件里重复定义后端结构。

## Emits

```ts
const emit = defineEmits<{
  (e: 'update:page', page: number): void
  (e: 'confirm'): void
}>()
```

- 112 个文件用类型化 `defineEmits<>`，沿用这个写法。
- 双向绑定统一走 `update:xxx` + `v-model:xxx`（115 个文件用到 `v-model`）。
  `defineModel` 目前只有 1 处，**不作为默认选择**——保持与既有代码一致用 `update:xxx`。

---

## 样式：Tailwind 优先

- `tailwind.config.js` 里 `darkMode: 'class'`，并扩展了品牌色板：
  `primary`（teal 系）、`accent`、`dark`，以及 `shadow-glass` / `shadow-card` /
  `shadow-glow`、`animate-fade-in` / `slide-up` / `scale-in` 等自定义动效。
- **用 `primary-*` 而非硬编码 `teal-*`**，换主题色时才不会漏改。
- 269 个组件带 `dark:` 变体。**新组件必须同时给出亮色和暗色样式**，
  漏 `dark:` 的表现是暗色模式下白底白字。
- 只有 49 个文件写了 `<style scoped>`，用于 Tailwind 表达不了的场景（复杂选择器、
  第三方库覆盖、`@keyframes`）。**新组件默认不写 `<style>`**；确需写时必须加 `scoped`。
- 全局样式与语义类（`stat-card`、`stat-value` 等 `@apply` 组合）在
  `src/style.css` / `src/styles/`，跨组件复用的视觉模式加在那里而不是各自复制类名串。

---

## 组件通信

| 场景 | 手段 |
|------|------|
| 父 → 子 | props |
| 子 → 父 | emits（`update:xxx` 用于 v-model） |
| 内容注入 | 具名 slot；表格用 `#cell-{key}` 作用域插槽（见 `DataTable.vue`） |
| 跨层级共享状态 | Pinia store（见 [State Management](./state-management.md)） |
| 跨页面一次性信号 | `window.dispatchEvent(new CustomEvent(...))`——仅限 `client.ts` 已有的 `ops-monitoring-disabled` / `admin-compliance-required` 这类全局事件，业务组件不要新造 |

**不要用 `provide/inject` 传业务数据**，当前代码没有这个模式。

---

## 弹层与过渡

- 弹层用 `<Teleport to="body">`（22 个文件），避免被父级 `overflow` / `transform` 截断。
- 基础对话框复用 `components/common/BaseDialog.vue`、确认框用 `ConfirmDialog.vue`，
  不要每个页面自己写遮罩层。
- 进出场动画用 `<Transition>`（13 个文件）配 Tailwind 的 `animate-*`。

---

## 文案与图标

- **模板里禁止硬编码用户可见文案**，一律 `{{ t('xxx.yyy') }}`：

  ```ts
  import { useI18n } from 'vue-i18n'
  const { t } = useI18n()
  ```

  245 个 `.vue` 已经这么做。细则见 [i18n Guidelines](./i18n-guidelines.md)。
- 图标统一走 `components/icons/Icon.vue`（`<Icon name="arrowUp" size="xs" />`），
  平台/模型图标用 `PlatformIcon.vue` / `ModelIcon.vue`，不要在组件里内联新的 SVG。

---

## 可访问性

现有基础组件已具备的实践，新组件照做：

- 装饰性图标加 `aria-hidden="true"`。
- 按钮/可交互元素用真实 `<button>`，不要给 `<div>` 挂 `@click`。
- 弹层支持 Esc 关闭与点击遮罩关闭（`BaseDialog` 已实现，直接复用）。
- 表单控件与 `<label>` 关联；纯图标按钮加 `aria-label` 或 `title`。
- 截断文本给 `:title` 保留完整值（见 `StatCard` 的 `:title="String(formattedValue)"`）。

---

## 富文本与 XSS

全仓库只有 12 处 `v-html`，每一处都必须经过消毒：

| 内容类型 | 做法 | 示例 |
|---------|------|------|
| Markdown（公告、合规文档、自定义页） | `marked` 渲染后 `DOMPurify.sanitize()`，结果放 `computed` | `AnnouncementPopup.vue`、`LegalDocumentView.vue`、`CustomPageView.vue`、`AdminComplianceDialog.vue` |
| 用户/管理员提供的 SVG 图标 | `sanitizeSvg()`（`src/utils/sanitize.ts`，`USE_PROFILES: { svg: true }`） | `AppSidebar.vue`、`ImageUpload.vue` |

**禁止把后端或用户输入直接绑到 `v-html`。** 新增 `v-html` 时，要么复用 `sanitizeSvg`，
要么照抄公告组件的 `marked` + `DOMPurify` 组合。

`src/i18n/index.ts` 设了 `warnHtmlMessage: false`（引导步骤的富文本是内部定义的），
这不代表可以在文案里放外部 HTML。

---

## 大页面的拆分手法

管理端页面容易膨胀。本项目的既定做法是**把纯计算逻辑抽成同目录 `.ts`**，
而不是无限拆子组件：

```
views/admin/GroupsView.vue
views/admin/groupsModelsList.ts          ← 可单测的纯函数
views/admin/groupsReasoningEffort.ts
views/admin/__tests__/groupsModelsList.spec.ts
```

状态类逻辑则抽成 composable（见 [Composable Guidelines](./composable-guidelines.md)）。
只有当一段模板真正需要复用或独立性很强时，才拆成子组件。

---

## 常见错误

- **忘了 `dark:` 变体** —— 暗色模式下不可读。
- **硬编码中文/英文文案** —— 切语言时残留。
- **在 `common/` 放业务组件** —— 该目录必须领域无关。
- **`v-if` 与 `v-for` 写在同一元素上** —— ESLint 里该规则被关掉了（历史原因），
  但**新代码不要这么写**，用外层 `<template v-if>` 包裹。
- **组件里直接 `axios`** —— 必须走 `@/api`（见 [API Layer](./api-layer.md)）。
- **在组件内重复声明后端返回类型** —— 从 `@/types` import。
- **`<style>` 不加 `scoped`** —— 污染全局。
