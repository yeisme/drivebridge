# client-shell Specification

## Purpose
定义个人文件客户端的视窗、导航分组、上下文路由和可安装网页壳。统一网盘、资源态势、访问凭据与管理页入口，保留本机 view 和普通网页的适用边界。

## Requirements

### Requirement: Locked client viewport

已登录工作台 SHALL 锁定为视窗高度，外层不得出现文档级滚动；侧栏、主内容与检查器各自滚动。登录页与本机一次性 view 不受此约束。

#### Scenario: Logged-in workspace does not bounce the window

- **WHEN** 用户在桌面宽度打开已登录工作台并滚动文件列表或用量页
- **THEN** 顶栏与侧栏保持在视口内，只有主内容或检查器滚动

### Requirement: Grouped navigation that can grow

主导航 SHALL 分为工作台、协作、系统三组，组可折叠并记住非敏感偏好；折叠主导航时仍以互不相同的图标与 `aria-label` 到达每个入口。新页面 SHALL 归入既有组，不得在侧栏底部无分组追加。

#### Scenario: User collapses a navigation group

- **WHEN** 用户收起「系统」组
- **THEN** 存储连接、Agent 授权与访问凭据从展开列表隐藏，组标题仍可展开，刷新后偏好保持

#### Scenario: Collapsed icon rail stays distinguishable

- **WHEN** 用户折叠主导航或使用 Ctrl/Cmd+B
- **THEN** 每个入口仍有独立图标与 `aria-label`，组标题不挡住图标

### Requirement: Resource status and access credentials as secondary tabs

总览与用量 SHALL 共用「资源态势」入口，用二级 Tab 切换；用户目录与 Key 管理 SHALL 共用「访问凭据」入口。旧路由 SHALL 打开对应 Tab，不得 404。

#### Scenario: Old analytics hash still opens usage

- **WHEN** 用户打开 `#/analytics`
- **THEN** 资源态势页可见，用量 Tab 被选中

#### Scenario: Old users hash still opens keys

- **WHEN** 用户打开 `#/users`
- **THEN** 访问凭据页可见，Key Tab 被选中

### Requirement: Precision Pine chrome

壳层 SHALL 使用 `style.css` 令牌绘制表面阶梯、微边框、投影与等宽数字；主按钮可有 1px 内高光，不得使用紫渐变、大面积装饰渐变或第二套主题色。

#### Scenario: Primary action uses the shared material

- **WHEN** 用户看到「连接存储」或「授权 Agent」主按钮
- **THEN** 按钮颜色来自 `--green` / `--green-hover` 令牌，数字指标使用 tabular nums

### Requirement: Installable overlay without a native shell

服务端网页 SHALL 声明 `window-controls-overlay` 回退 `standalone`，并提供网盘与传输的桌面快捷方式；本机 `drivebridge view` 仍不得声明安装清单；不得为此引入 Tauri、Wails 或 Electron。

#### Scenario: Manifest prefers overlay

- **WHEN** 浏览器读取 web app manifest
- **THEN** `display_override` 含 `window-controls-overlay`，且仍提供 `standalone`

#### Scenario: Local view stays ephemeral

- **WHEN** 用户打开 `drivebridge view` 的本机只读视图
- **THEN** 该页不声明安装清单

### Requirement: Standalone management navigation

客户端 SHALL 在“系统”组提供独立“管理与授权”页面，原工作区菜单和用户目录的管理入口 SHALL 到达该页面。页面 SHALL 支持 `#/governance` 的 tab、tenant 与管理空间上下文，刷新和历史导航保持当前分类。

#### Scenario: Open a management deep link

- **WHEN** 用户打开 `#/governance?tab=events&tenant=t&space=s`
- **THEN** 在原应用壳内展示有权访问的管理页面及访问记录分类，保持 t/s 的上下文，URL 不授予额外权限

#### Scenario: Unknown category and forbidden tenant

- **WHEN** tab 不被识别或用户无权访问指定租户
- **THEN** 未识别 tab 回到申请分类；无权访问租户明确拒绝，不退回到权限更大的工作区

#### Scenario: Legacy navigation still works

- **WHEN** 用户通过旧用户、Key、团队、网盘、分享或资源态势链接进入
- **THEN** 原链接仍到达原能力，新增管理页不替换它们的数据归属

### Requirement: Structured workspace menu and settings outline
工作区菜单 SHALL 按身份、空间和导航分组，体验身份控件不挤压标签。设置页 SHALL 以大纲替代右侧使用偏好说明卡，保留实际外观和资料能力。

#### Scenario: Navigate settings sections
- **WHEN** 用户点击或键盘激活大纲项目
- **THEN** 对应分区进入视口并可获得焦点，现有 hash 路由和未保存字段不变

#### Scenario: Switch an experience persona
- **WHEN** 用户打开工作区菜单并选择体验身份
- **THEN** 标签、选择器与确认动作保持可读，提交沿既有身份切换和页面清理流程
