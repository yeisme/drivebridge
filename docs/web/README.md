# 文档索引

- [个人网盘与 Agent 工作台 PRD](product/personal-agent-workspace-prd.md)：当前产品目标、R01–R13 能力账本、双端旅程与 S0–S5 交付切片。
- [当前 OpenDesign 核对](ui/personal-workspace-reference.md)：2026-09-30 的主资源 v=33、截图、源码差异和证据边界；旧画面的 v9.2 标签不作为版本依据。
- [新版 UI 规格](ui/personal-workspace-ui-spec.md)：逐页控件、tokens、导航/详情、手机交互、数据与异常状态。
- [多笔记工作台正式实现规格](ui/notebook-workspace-production-spec.md)：正式 Web 实现的笔记布局、渲染、Obsidian 式交互、动效和 Pinax adapter 边界。
- [多笔记工作台实现交接](../delivery/notebook-workspace-implementation-handoff.md)：原型证据与正式开发顺序。
- [前端清理与开工计划](ui/personal-workspace-rebuild.md)：先拆旧壳，文件级保留/替换清单、旧路由兼容和真实验证命令。
- [新版验收矩阵](ui/personal-workspace-acceptance.md)：AC01–23 与前后端任务/测试层关联。
- [前端 OpenSpec](../../openspec/changes/drivebridge-personal-workspace-rebuild-v1/README.md) / [后端合同与任务](../../openspec/changes/drivebridge-personal-workspace-contracts-v1/README.md)：设计已落盘，代码实施未开始。

- [DriveBridge 双端产品原型](ui/personal-agent-prototype.md)：v9.2 全页面紧凑列表与状态行、单元格选中及锚定浮层、分级弹层、文档／资源分页；保留资源树、导航历史、统一检索、收藏、Markdown 属性、外部 MCP/API 和产物验收。真实服务及原生能力独立验收。

- [文件平台演进规格与文档](file-platform-evolution.md)：待实施的能力边界、流程、owner任务与验证入口。
- [信息工作台（总览·文件详情·钻取）](file-platform-evolution.md)：`#/overview` 总览六组面板、同口径钻取与文件详情六页签；实现状态与回滚见该文档验收节。规格归 `openspec/changes/drivebridge-information-workspace-v2/`。
- [文件流程（上传恢复·检索·整理·交付与讨论）](file-platform-evolution.md)：`#/flows` 四中心与能力门、幂等恢复身份与分阶段进度；实现状态与回滚见该文档验收节。规格归 `openspec/changes/drivebridge-file-platform-consumers-v2/`。

网页消费 DriveBridge API。设计、实现和本地验收归 `openspec/changes/archive/2026-09-11-drivebridge-personal-web-v1/`；密钥登录与用户密钥管理页归 `2026-09-15-drivebridge-key-login-users-admin-v1`（key-only 部署浏览器可达）。绿色工作区体验归 `2026-09-17-drivebridge-green-workspace-v2`，多租户治理 Web 面归 `2026-09-17-drivebridge-multitenant-governance-v1`。个人楔子核文件/传输/收件箱/Agent 授权；治理与团队是 committed 管理面，不是本阶段主叙事。

运行、功能范围与验证命令见 [README](../../web/README.md)。视觉令牌、产品标与 Finder 交互见 [UI 设计](ui/DESIGN.md)。壳层视窗、分组导航、图表原语与 PWA overlay 见 [客户端壳](ui/client-shell.md)，实现归 `drivebridge-client-shell-v1`。Key 删除与回收站批量见 `drivebridge-key-delete-trash-ui-v1`。传输/收件箱/Agent/连接/分享列表手感见 `drivebridge-page-interaction-v1`。

- [本轮验证与截图](verification.md)
- [筛选、身份信息与菜单交互改进](ui/interaction-polish.md)：第三轮六处标注、高级筛选浮层、审计名称、Key 菜单和设置大纲。
- [紧凑工作区验收记录](ui/compact-workspace-validation.md)：门禁、截图对照、失败修复归因与本地预览边界。
- [紧凑工作区与可读性规范](ui/compact-workspace.md)：两轮十条标注、控件与内容间距、独立管理页、列表详情及分栏/画廊阅读验收。
- [统一存储可视化设计](unified-storage-visualization.md)
- [跨平台 client 支持设计](cross-platform-client.md)
- [文件阅读器与团队测试交付](ui/file-readers.md)
- [文件预览、图片批注、团队评论与机器人接入](ui/preview-collaboration.md)

用量、图表、扫描与安全审计见[使用说明](insights-audit.md)。

- [多租户管理与授权中心](tenant-governance.md)

- [团队工作区](team-workspace.md)：成员、网盘、权限、存储分配、归档与隔离体验。

## Agent Skills

本项目会话的 active skills 由仓库根 `.skills/profiles/targets/cli/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target cli/drivebridge` 生成 `.agents/skills/` 与 `.claude/skills/`。不要手改运行副本。

完整对照、缺口与下一波优化见 [子项目 Skill Profile](../../../../docs/skills/subproject-skill-profiles.md)。
