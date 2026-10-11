# visual-craft Specification

## Purpose
定义全站共享的墨绿色主题、文字层级、语义边框、控件尺寸、图标和内容留白。区分单行控件与复合内容，确保深浅主题、键盘焦点、文件选择及拖拽状态一致且可读。

## Requirements

### Requirement: Shared surface tokens

客户端 SHALL 在共享主题入口提供表面、文字、边框、投影与半径语义令牌，现有布局别名 SHALL 指向同一来源。Insights、Finder、表格与表单 SHALL 引用这些令牌，不得新增独立主题或让分隔线回退到文字 currentColor。

#### Scenario: Insight card uses shell tokens

- **WHEN** 用户打开资源态势
- **THEN** 指标与图表区域的背景、边框和投影来自共享语义令牌，深浅主题同步切换

#### Scenario: Dark table separators do not become bright lines

- **WHEN** 用户在深色主题打开用户目录或团队权限表
- **THEN** 普通行分隔线使用共享边框色，不呈现白色或正文色亮线；键盘焦点仍独立可见

### Requirement: Status pills and numeric alignment

状态与用量数字 SHALL 使用共享胶囊与 tabular nums；状态点不得用 emoji。

#### Scenario: Transfer dock uses tabular counts

- **WHEN** 顶栏显示传输数量
- **THEN** 数字以等宽数字渲染，状态点来自 CSS 而不是 emoji

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

### Requirement: Control size and content spacing are independent

客户端 SHALL 将桌面单行控件统一为 32px，并给业务表格至少 48px 行高、单元格上下各至少 8px 与左右各至少 12px 留白。双行记录 SHALL 至少 64px 且随内容增高。复合文件项、预览和多行内容 SHALL NOT 继承单行控件固定高度。

#### Scenario: Inline permission controls keep breathing room

- **WHEN** 权限表行包含 32px 下拉控件
- **THEN** 控件上下各有至少 8px 内容留白，文字和控件均不触及行边界

#### Scenario: Narrow table remains readable

- **WHEN** 在 872px 或 390px 窗口查看含长姓名、状态与操作的表格
- **THEN** 不压缩正文到 14px 以下或裁切多行内容；仅表格自身可以水平滚动，工具栏距外边框至少 16px

### Requirement: Form spacing and explicit actions

客户端 SHALL 保留可见字段标签，标签与控件间至少 8px、字段组间至少 16px、字段区到操作区至少 24px、取消与提交间至少 8px。字段错误 SHALL 就近展示，失败保留输入；触屏交互目标 SHALL 至少 44×44px。

#### Scenario: Storage assignment form actions do not touch inputs

- **WHEN** 用户打开团队“分配已有存储”表单
- **THEN** 下拉、取消与保存处于独立区域，有规定间距且全部可达，提交不会因误触相邻按钮重复执行

#### Scenario: Failed submission preserves user input

- **WHEN** 提交返回校验或权限错误
- **THEN** 表单保留已填内容，在当前弹层显示错误，用户可修改重试或取消
