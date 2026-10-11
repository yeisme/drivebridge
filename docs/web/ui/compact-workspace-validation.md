# 紧凑工作区本地验收记录

## 交付范围

两轮十条浏览器反馈已落实到共享控件、独立管理页、Finder、资源态势、用户与团队页面。墨绿色品牌与深浅主题保持，HTTP/MCP、权限判定、一次性审批执行及文件固定版本合同未改变。实现与任务由本项目 `drivebridge-compact-workspace-v1` 管理。

桌面单行控件为 32px；触屏控件至少 44px。普通业务行至少 48px，双行信息至少 64px，单元格上下 8px、左右 12px；表单操作区保留 24px 距离。管理页支持五分类深链接及 480px 按需抽屉。网盘、团队提供默认列表详情及卡片切换；分栏和画廊保留选择并明确提供完整阅读入口。

## 自动化门禁

所有下列证据路径均相对项目根目录，各目录含 runner 生成的 `summary.json`、命令、日志、脱敏环境和 `artifacts/`。

| 检查 | 结果 | 证据目录 |
| --- | --- | --- |
| `pnpm test` | 28 个文件、123 项通过 | `temp/integration-test-runs/20260922T141705Z-6134238c/` |
| `pnpm typecheck` | UI 原语/样式扫描及 TypeScript 通过 | `temp/integration-test-runs/20260922T140859Z-92a0b6f9/` |
| `pnpm build` | TypeScript 与 Vite 构建通过 | `temp/integration-test-runs/20260922T140826Z-b2e7a71e/` |
| `pnpm test:e2e` | 普通 fixture 41 项通过；19 项属于下列专用 fixture | `temp/integration-test-runs/20260922T140901Z-fa6b4253/` |
| 治理 fixture | 5 项通过：管理只读、审批后单独执行、逐项回执、账号恢复入口、团队管理对接 | `temp/integration-test-runs/20260922T140901Z-a77c2808/` |
| 团队布局与范围 | 4 项通过：管理深链与失败表单、表格留白、团队导航布局、角色/管理空间 | `temp/integration-test-runs/20260922T132124Z-5a45b7ee/` |
| 团队生命周期与配额 | 4 项通过：创建、邀请、交接、归档恢复、配额调整 | `temp/integration-test-runs/20260922T133330Z-266725bb/` |
| 用户生命周期 | 1 项通过：过滤、详情、Key 元数据、冻结/恢复/归档 | `temp/integration-test-runs/20260922T132630Z-0a5849f9/` |
| 创建与身份切换 | 3 项通过：向导、直接入口、Key 轮转后的体验身份切换 | `temp/integration-test-runs/20260922T134612Z-b070c2ef/` |
| 管理员接管 | 1 项通过：确认目标身份、最后负责人职责接管 | `temp/integration-test-runs/20260922T134612Z-d6637aeb/` |
| 无账号体验 | 1 项通过：进入、授权写入、退出 | `temp/integration-test-runs/20260922T140309Z-5f434a31/` |
| 动效策略 | 通过，0 警告 | `temp/integration-test-runs/20260922T140337Z-de980598/` |
| `openspec validate --specs --strict --no-interactive` | 16 项正式规格通过，0 失败 | `temp/integration-test-runs/20260922T145131Z-eea69bbc/` |

普通 fixture 的 19 个 skipped 不是通过数；它们按所需权限/身份在上表专用服务中运行。接管、用户归档等破坏性生命周期场景各自启动一次性 fixture，避免前一场景冻结/删除身份影响后续测试。均未连接真实 OneDrive 或付费服务。

本机 view 回归使用真实本机 owner 和临时文件树。当前 sibling owner 的依赖图需要更新，因此浏览器测试复制其 `go.mod`/`go.sum` 到临时目录，使用 `go build -mod=mod -modfile` 构建；最终图保存在对应 `artifacts/local-viewer.mod` 与 `.sum`，未修改 owner 的文件。此验证不代表 owner 原始 lockfile 已修复。

## 布局、状态与安全语义

- 实测宽度为 390、872、882、957、1213、1280、1440px，高 963px；Finder、概览、容量覆盖深浅主题及侧栏两种状态。管理页、用户目录、权限表、表单覆盖同样宽度和两种主题。
- 以真实 bounding box 和 computed style 检查按钮同排 32px、高差不超过 1px、表格行高、8/12px padding、24px 表单留白、语义边框及页面无横向滚动。宽表和分栏内部滚动允许；容器 640/880px 断点由内容宽度决定。
- 触屏用例在 390/957px 检查 44×44px 点击区域；文件设计回归检查正文对比度至少 4.5:1。减少动效策略保持内容和焦点，截图关闭未完成的有限动画，避免把旧 Tab 渐退状态误作当前选中态。
- 容量单测覆盖真实零、未采集、全部未采集、未知配额和键盘下钻；真实零只保留零基线，缺口无密集圆点。
- 抽屉验证 Escape/取消后的焦点恢复、403 后保留输入、租户切换后关闭旧表单；原 API 上下文 revision 拒绝陈旧结果。普通成员、管理只读与跨账号访问继续由服务端判定。
- 真实审批回归验证“批准”不修改文件，只有申请人“确认执行清单”后文件才进入回收站。partial/interrupted 展示逐项回执和重新申请指导，原清单没有重放入口；传输中断沿原 operation 恢复。
- 分栏/画廊覆盖 100 字中文名称、四种宽度、Enter 与完整文本阅读；原图片、PDF、音视频、HTML 沙箱、网络失败重试及旧版本阅读回归通过。并行任务中的表格文件预览修改保持原样，不将其实现计入本轮 UI 改造。

## 截图对照

改造前统一基线：`temp/integration-test-runs/20260922T095518Z-6f28ea4b/artifacts/`。

| 标注 | 改造前 | 改造后 |
| --- | --- | --- |
| A1 管理中心 | `before-governance-957.png` | `20260922T140309Z-8d0a797c/artifacts/after-governance-light-957.png`、`after-form-light-957.png` |
| A2 工具栏尺寸 | `before-finder-dark-957.png` | `20260922T140901Z-fa6b4253/artifacts/after-finder-dark-957-false.png` |
| A3 位置描述/专注 | `before-finder-light-957.png` | `20260922T140901Z-fa6b4253/artifacts/after-finder-light-957-false.png` |
| A4 容量排版 | `before-capacity-dark-957.png` | `20260922T140901Z-fa6b4253/artifacts/after-capacity-dark-957-false.png` |
| A5 概览 | `before-overview-dark-957.png` | `20260922T140901Z-fa6b4253/artifacts/after-overview-dark-957-false.png` |

“改造后”路径均位于 `temp/integration-test-runs/` 下。第二轮截图以用户提供的五张标注图为输入，新增结果为：

- B1/B2/B3：`20260922T140309Z-8d0a797c/artifacts/after-permissions-dark-872.png`、`after-users-dark-872.png`、`after-storage-form-dark-872.png`。
- B4：`20260922T140901Z-fa6b4253/artifacts/directory-detail-872.png` 与 `green-drive-home.png`；目录返回、有效选择、卡片切换另有行为断言。
- B5：同目录 `reader-columns-1213.png`、`reader-gallery-1213.png`，另含 957、872、390px 图。缩略图、名称与阅读按钮不继承普通按钮固定行高。

## 失败记录与修复归因

- 初次新账号没有绑定空间，测试等待网盘失败：改用已有空间的离线身份，并在需要空目录时通过真实 API 创建专属测试空间。未提高等待时长或放宽权限。
- 紧凑工具条同时显示宽/窄操作：修正 container rule 与原语 display utility 的层级；“更多”原先丢失文件选择：传入当前选择，使分享仍针对选中文件。
- 管理 Tab 内容挤在导航右侧：修正 Base UI 的 orientation 属性选择器。触屏收纳按钮原宽 32px：通过控件专属 min-size 保证至少 44px。
- 全量测试暴露旧搜索角色、折叠的引用区和负责人标签定位；已更新到真实可访问控件。身份切换测试等待实际页面 load 后再导航，避免在旧页面点击。
- HTML 预览在共用目录的一次全量运行中未等到新文件，单测重跑通过；最终用例改为专属空空间并等待目录加载完成，完整回归通过。该现象按 fixture 初始化/隔离修复记录，不推断为后端权限缺陷。

历史失败日志继续保留在各自 run 目录；成功证据不覆盖失败证据。

## OpenSpec 收口

16 项任务全部完成；原语前置 change 与本 change 已按依赖顺序归档。九个 capability 的正式规格合入 11 项新增要求、7 项修订要求。归档证据为 `temp/integration-test-runs/20260922T141737Z-e0755ab2/`；[归档任务](../../../openspec/changes/archive/2026-09-22-drivebridge-compact-workspace-v1/tasks.md) 保留真实浏览器通过证据。

归档后的严格校验发现 16 个正式规格仍保留自动生成的 Purpose 占位句；已按各自既有要求补齐用途说明，未改动额外业务要求。重新严格校验全部通过，归档后的文档相对链接也已修复。

## 本地预览与交付边界

本地服务继续使用已有隔离体验实例，静态资源更新后刷新浏览器可见。服务端无需迁移；本任务未发布、未部署，也未主动执行提交或推送。工作期间仓库的定时收口自动产生了 `f791f47`（含本轮与并行任务文件），本任务保留该提交。表格文件阅读的并行工作保留，未作为本 change 归档。正式上线仍需其独立发布流程。
