## ADDED Requirements

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
