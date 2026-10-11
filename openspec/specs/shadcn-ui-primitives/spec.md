# shadcn-ui-primitives Specification

## Purpose
统一以现有 shadcn/Base UI 原语构建客户端控件、弹层、选择器和表格，使用共享语义主题。明确键盘、视口适配及 jsdom 与真实浏览器测试的分工，避免页面自行复制交互基础设施。

## Requirements

### Requirement: Official primitive foundation
The client MUST build creation flows and controls on the official shadcn Base Nova primitives under `src/components/ui/`, themed through semantic tokens that follow the `data-theme` dark/light switch.

#### Scenario: Theme follows semantic tokens
- **WHEN** the user switches 深色/浅色主题 and reloads
- **THEN** dialogs, selects and tables rendered from the official primitives keep consistent colors without per-component overrides

### Requirement: Creation flows on primitives
Dialogs MUST go through `AppDialog` (Base UI Dialog) and single-choice filters through `SelectField` (Base UI Select); migrated selects are not native `<select>` elements.

#### Scenario: Open a migrated creation dialog
- **WHEN** the user opens 批量签发 or 新建文本文件
- **THEN** the dialog is reachable by role `dialog` with its title as the accessible name and closes on 确认/关闭

#### Scenario: Choose from a migrated select
- **WHEN** the user clicks a migrated combobox such as 筛选存储 or Key 状态 and picks an option
- **THEN** the trigger shows the chosen label and the filtered result updates

### Requirement: Keyboard and viewport parity
Global keyboard shortcuts MUST keep working while a Base UI popup is open, and popup bounds MUST settle inside the viewport on mobile.

#### Scenario: Arrow keys navigate quicklook inside the dialog
- **WHEN** the quicklook dialog is open and the user presses ArrowDown/ArrowUp
- **THEN** the previewed file switches even though the popup stops propagation of composite keys

#### Scenario: Mobile viewport bounds
- **WHEN** the viewport is resized to phone width while a popup is open
- **THEN** the popup asynchronously settles within the viewport bounds

### Requirement: jsdom test boundary
Component unit tests MUST NOT drive Base UI popup open/close choreography under jsdom (it starves the event loop); popup interaction is verified in real-browser e2e and filter logic through pure functions.

#### Scenario: Unit scope for migrated selects
- **WHEN** a view migrates a filter to SelectField
- **THEN** its jsdom test asserts the combobox renders and exercises the filtering through a pure function, while tests/shadcn-controls.spec.ts covers the popup interaction in a real browser

### Requirement: 共享弹窗按控件语义布局表单
客户端 SHALL 在共用弹窗内为普通与嵌套字段提供上方标签和 8px 间距，为勾选标签提供横向布局；触屏勾选区域至少 44px，创建表单提供明确底部操作区。

#### Scenario: 机器人授权表单
- **WHEN** 用户打开授权机器人弹窗
- **THEN** 操作选项与复选框在同一行，默认只勾选读取文件，取消后恢复入口焦点

#### Scenario: 嵌套存储字段
- **WHEN** 用户打开连接存储弹窗
- **THEN** 双列内的存储桶及区域字段标签与输入保持 8px 间距，窄屏正常排列
