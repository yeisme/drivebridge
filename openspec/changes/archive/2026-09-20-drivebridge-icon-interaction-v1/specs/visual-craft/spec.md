## ADDED Requirements

### Requirement: Unified product mark

客户端 SHALL 在侧栏、登录页、favicon 与 PWA 图标中使用同一座桥形产品标，并提供 maskable 安全区资产。

#### Scenario: Sidebar shows the product mark

- **WHEN** 用户打开已登录工作区
- **THEN** 侧栏品牌位渲染产品标 SVG，而不是通用文件 lucide 图标

#### Scenario: Installed app uses a maskable icon

- **WHEN** 浏览器读取 web app manifest
- **THEN** icons 列表包含 `purpose` 含 `maskable` 的条目

### Requirement: Distinct navigation icons

折叠或展开的主导航 SHALL 为总览、用量、网盘、传输、收件箱、分享、存储、Agent、团队、用户目录、Key 与回收站使用互不相同的图标。

#### Scenario: Collapsed sidebar remains distinguishable

- **WHEN** 用户折叠主导航
- **THEN** 每个入口仍有独立 `aria-label` 且图标图形互不相同

### Requirement: File kind glyphs

无缩略图时，客户端 SHALL 按文件类型显示对应 lucide 图标，音频与 PDF 不得显示为普通文档图标。

#### Scenario: Audio file without thumbnail

- **WHEN** 文件 MIME 以 `audio/` 开头且缩略图不可用
- **THEN** 回退图标为音频类型，而不是文档类型

### Requirement: Finder menu scanability

文件操作菜单 SHALL 为主要动作提供图标，用分隔线分组，危险动作单独成组，并在已有快捷键的项上显示快捷键。

#### Scenario: File context menu on a pointing device

- **WHEN** 用户对文件打开操作菜单
- **THEN** 预览、下载、复制、重命名、回收站等项可见，回收站为危险样式，且复制项显示快捷键

#### Scenario: Coarse pointer uses an action sheet

- **WHEN** 在粗指针或窄屏上打开文件操作菜单
- **THEN** 菜单作为贴近视口底部的 action sheet 出现，且不超出视口

### Requirement: Keyboard file selection without a persistent checkbox

Finder SHALL 支持方向键移动焦点、Enter 打开或预览、Esc 清除选择；列表左侧 checkbox 在细指针下默认不常驻显示。

#### Scenario: Keyboard selects and previews a file

- **WHEN** 文件列表聚焦且用户按向下键再按 Enter
- **THEN** 焦点移到下一项并打开预览，无需点击 checkbox

#### Scenario: Arrow keys navigate folders

- **WHEN** 焦点在文件夹上且用户按向右键
- **THEN** 进入该文件夹

- **WHEN** 用户在子目录按向左键
- **THEN** 返回上级目录

### Requirement: Visible drop targets

将文件拖到文件夹或目录树节点时，客户端 SHALL 显示明确的落点反馈。

#### Scenario: Dragging a file over a folder

- **WHEN** 用户把文件拖到另一个文件夹上
- **THEN** 该文件夹呈现落点高亮，松手后按现有移动合同处理
