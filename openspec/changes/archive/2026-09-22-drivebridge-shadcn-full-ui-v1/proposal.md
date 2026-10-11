# 全站迁移官方 shadcn Base Nova

## Why

客户端此前用原生控件加自写 CSS（forms.css / workspace-layout.css 的 legacy 层）拼装界面，创建流程（对话框、选择器、表单）与数据表格的可访问性、键盘行为和深浅色一致性都要逐处手写。官方 shadcn Base Nova 组件库（Base UI 原语 + 语义令牌）提供可主题化的对话框、选择器、数据表格等基元，把「绿色双主题」从令牌层贯穿到控件层。

## What Changes

- 引入官方 shadcn Base Nova 组件（`src/components/ui/`，经 `scripts/import-shadcn.py` 从 registry 导入）与 Tailwind v4 语义令牌，深浅色主题统一走 `data-theme`。
- 创建流程迁移：对话框统一走 `AppDialog`（Base UI Dialog），选择器统一走 `SelectField`（Base UI Select），替换原生 `<select>` 与旧的弹层实现。
- 数据表格迁移到官方表格基元（`data-table.tsx` + @tanstack/react-table），创建/管理流程逐步接入。
- 全站视图（工作台、Finder、交付、团队、治理、目录、媒体、洞察）逐屏迁移；完成后清理旧 UI 层（legacy forms.css 规则与被替换的旧组件样式）。
- 键盘与可访问性对齐：全局快捷键在 Base UI 弹层内仍可用（捕获阶段监听），弹层尺寸在移动端收敛于视口内。

## Capabilities

### Added Capabilities

- `shadcn-ui-primitives`：官方原语基线、jsdom 测试边界与键盘/移动端行为契约。

### Modified Capabilities

- `green-workspace`：控件层从原生/自写样式切换到语义令牌驱动的官方原语（令牌语义不变）。
