# 客户端壳

2026-10-01 当前桌面决定：用户指定 Electron，同仓 `desktop/` 复用当前 Web UI，不新建 Reader。完整宿主/安全/生命周期以 [Electron 架构](../../architecture/electron-desktop.md) 为准，Pinax 三屏以 [知识阅读规格](knowledge-reading-spec.md) 为准。本文其余旧 Web 壳样式/尺寸只适用于对应历史阶段；与当前 personal-workspace UI Spec 冲突时以新规格为准。桌面仍待实施，不能用旧 PWA 证据标记通过。

2026-09-27：未来产品方向已扩展为桌面与手机独立客户端并重，见 [我的网盘 + Agent 双端原型](personal-agent-prototype.md)。本文其余内容保留现有 Web/PWA 壳层的实现依据，不代表原生客户端已经实现。

个人文件工作台的壳层，不是运营后台。视觉令牌与 Finder 手感见 [DESIGN.md](DESIGN.md)。本文件约束视窗、导航分组、图表原语和「浏览器即 client」的扩展边界。已归档的 `drivebridge-client-shell-v1` 是基础版本；当前增量见 [紧凑工作区与可读性规范](compact-workspace.md)，验证结果见 [本轮验收记录](compact-workspace-validation.md)。

## 姿态

看起来像装在 Dock 里的文件客户端：视窗锁定、一行工具栏、分组导航、可扫读图表。不是 CMS、不是营销页、不是多窗口原生壳。

风格名：Precision Pine（墨林精工）。主色仍是 `--green`。新页面先问「它属于哪一组、用哪套令牌」，不要另起主题。

## 视窗

已登录 `.app`：`height: 100dvh; overflow: hidden`。滚动只发生在侧栏、主内容、检查器。顶栏最小高度 56px，不随内容离开视口；窄屏操作栏换行时自然增高，不能遮挡内容。展开侧栏 224px，折叠 64px。页面位置由面包屑说明，保留无障碍标题，不再重复显示大标题、描述与品牌页脚。

登录页和 `drivebridge view` 一次性页面保持普通文档流。

## 导航分组

| 组 | 入口 | 说明 |
| --- | --- | --- |
| 工作台 | 资源态势、我的网盘、传输中心、产物收件箱 | 日常文件与用量 |
| 协作 | 分享、团队、回收站 | 人与外链 |
| 系统 | 存储连接、机器人授权、访问凭据、个人设置、管理与授权 | 连接、身份与显式管理入口 |

资源态势统一为概览、容量、传输、机器人、安全审计一层 Tab，支持 view 与 space 深链参数。访问凭据 = 原用户目录 + Key 管理，二级 Tab。旧 hash `#/analytics`、`#/users` 必须打开对应 Tab。

组折叠记在 `drivebridge:nav-groups`，失败当全展开。主导航折叠（⌘B）变成图标 Rail，图标不得复用。窄屏保留现有紧凑图标导航，用量入口位于顶栏。

新能力先归组，禁止在侧栏底部无分组追加。

管理页为 `#/governance?tab=requests`，分类、租户和管理范围随链接恢复；工作区菜单和租户设置复用它。网盘/团队目录新增列表详情与保留的卡片模式，完整团队详情和文件路由不变。专注入口属于 Finder 视图工具区，移除重复的位置描述行。

侧栏空间用量打开独立查看弹层，配额与身份边界见 [空间配额与工作区](capacity-workspace.md)。账户按钮打开菜单，退出为明确操作。

## 图表

只用项目内 SVG（`src/insights/charts.tsx`）。禁止 ECharts、Chart.js、Recharts。

- 缺失日是缺口，不是零。
- 有效量与观测量并排，不叠成总量。
- 悬停/聚焦有跟随浮层；「查看数据表」仍在。
- 容量分段条在配额未知时不画满额。

口径与扫描规则仍见 [用量统计与安全审计](../insights-audit.md)。

## 客户端路径

浏览器即 client。三阶段只做记录，本 change 只交付第一阶段：

1. **现在**：PWA `standalone` + `window-controls-overlay` 回退、桌面 shortcuts（网盘、传输）。同一份 `dist/`。
2. **当前已选型、待实施**：Electron + Forge，复用既有 Web 构建物；Tauri/Wails 不再是未决选项。
3. **以后**：托盘、file_handlers、本机文件系统事件。需要单独 OpenSpec 与根边界审批。

Electron 由新桌面 change 实施，本机 view 不加安装清单。macOS、Windows、Linux 的新桌面实机项全部独立待验，旧 CLI/PWA 状态不自动升级。

## 非目标

本历史 Web 壳切片不实现原生宿主，当前 Electron 宿主归新桌面 change；仍禁止把治理状态搬进客户端或为装饰引入动画库。早期壳层切片曾将暗色主题等列为非目标；其后续交付以当前 [个人设置与主题](personal-settings.md) 及各 owner spec 为准，不再用历史排除项阻止已交付能力。
