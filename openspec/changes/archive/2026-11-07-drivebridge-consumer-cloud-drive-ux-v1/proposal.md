# DriveBridge C 端网盘体验与清透视觉全链路重构

## Why

当前 DriveBridge Web 与桌面共享界面采用深松绿后台侧栏（`#15221b`）、22 项平铺管理菜单、选盘中间页（`DriveHome`）、五层堆叠的 Finder 工具栏以及裸露协议枚举/64 位哈希的详情面，整体观感偏向 B 端运维控制台而非 C 端个人网盘产品。用户明确要求按 C 端网盘交互体验全链路重构与美化，采用清透果味网盘风、登录后直达个人主网盘文件、精简主导航并补齐框选、悬浮多选胶囊栏、全画布拖拽与右下角悬浮传输窗。

## What Changes

- **清透果味网盘视觉系统（Tokens & File Badges）**：将浅色侧栏由深松绿后台改为清透浅灰白（`#f4f6f5`）配纯白无框文件画布（`#ffffff`），深色模式同步柔和化；品牌松绿收敛为主操作按钮与激活点缀；升级彩色高辨识度文件类型徽标与可读文件类型名称，彻底清理移动端及详情中的裸 MIME 字符串。
- **直达主个人网盘与精简 C 端导航**：默认登录及 `#/files` 直开首个本人个人空间（保留 `#/drives` 空间总览及全部 22+ 旧 `#/` 路由兼容）；侧栏顶部设置醒目的「＋ 新建 / 上传」主入口，一级导航收敛为 7 项高频入口（全部文件含多盘一键切换、最近使用、收藏与图集、知识笔记、AI 投递箱、我的分享、回收站），低频管理模块统一收口至双栏「设置与空间管理」中心。
- **Finder 核心网盘交互与右下角传输悬浮窗**：合并顶栏与目录面包屑/搜索/视图切换，释放约 120px 首屏高度；移除列表行生硬直角绿框；新增桌面鼠标框选（Marquee Selection）、底部悬浮多选操作胶囊栏（`SelectionActionBar`）、全画布拖拽上传浮层（`DropzoneOverlay`）及右下角可折叠传输进度悬浮窗（`TransferDock`，严格保留五阶段耐久校验语义）。
- **沉浸式预览器、AI 投递箱与全站去后台术语化**：重做 `FilePreviewDialog` Lightbox 顶栏与右侧详情抽屉（底层 `file_id`/`version_id` 默认收进折叠面板，外露一键复制 AI 引用）；将 `SubmissionInbox` 升级为卡片式「AI 投递箱」，将 `WorkspaceHome` 升级为视觉化「最近使用」，清理全站内部枚举串与免责声明式生硬文案。

## Capabilities

### New Capabilities
- `consumer-cloud-drive-shell`：清透果味视觉令牌、直达默认个人网盘、顶部新建上传总入口、7 项精简导航与统一「设置与空间管理」中心。
- `consumer-finder-interactions`：双层极简文件头、彩色文件徽标与可读类型、鼠标框选、底部悬浮多选胶囊栏、全画布拖拽遮罩、右下角传输悬浮窗及沉浸式预览收口。

### Modified Capabilities
- 兼容并演进 `client-shell`、`finder-file-actions`、`visual-craft` 的前台表现层；不删除 PRD R01–R15 任何能力，不改动 Go `server/`、CLI/MCP 或 Electron `desktop/` IPC 公开合同，所有旧 `#/` 路由持续可达。

## Impact

影响范围限于 `cli/drivebridge/web/src/**`、`cli/drivebridge/web/tests/**`、`cli/drivebridge/docs/web/ui/**` 及本 OpenSpec 变更目录。服务端 API/GORM、MCP schema 与桌面宿主安全护栏保持零破坏兼容。
