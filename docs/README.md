# DriveBridge 文档

## Electron 与 Pinax 阅读设计

- [Electron 桌面架构](architecture/electron-desktop.md)：用户已选 Electron；同仓宿主、授权本机门面、Pinax 合同、受管范围与离线阅读，尚待实现。
- [知识阅读三屏](web/ui/knowledge-reading-spec.md)：首页、搜索、阅读及共用 reader/无障碍。
- [多笔记工作台正式实现规格](web/ui/notebook-workspace-production-spec.md)：标签页、分屏、Markdown 渲染、Obsidian 式交互、动效、私有状态与 Pinax 适配边界。
- [多笔记工作台实现交接](delivery/notebook-workspace-implementation-handoff.md)：当前原型证据、正式 Web 实现顺序、验证命令与禁止的实现捷径。
- [桌面阅读验收矩阵](delivery/desktop-knowledge-acceptance.md)：原 F/B 复用、新增 D/P 任务、三平台实机门与自然观察。
- [本轮设计核验](delivery/desktop-knowledge-design-verification.md)：strict、链接、任务保护与未验证事项；不是产品测试报告。
- [实施 OpenSpec](../openspec/changes/drivebridge-desktop-knowledge-reader-v1/README.md)：任务保持待办；不新建 Reader、不替代原 Web/PWA/移动需求。

## 统一开发导航

- [完整设计包与实施交接](delivery/design-handoff.md)：产品范围、统一架构、冻结原型、技术选型与分阶段实施入口；本次仅设计，不代表业务落地完成。

- [开发、构建、测试](development.md)：一个工作目录和一组命令。
- [个人多 Agent 工作台 PRD](web/product/personal-agent-workspace-prd.md)
- [工作台技术选型](architecture/personal-workspace-technical-decisions.md)：最新快照、单机优先、Go DTO 生成合同与现有栈取舍。
- [前端模块文档](web/README.md) / [服务端模块文档](server/README.md)
- [仓库迁移与历史保护](repository-consolidation.md)
- [统一 OpenSpec](../openspec/changes/archive/2026-09-30-drivebridge-monorepo-consolidation-v1/design.md)

- [文件平台演进规格与文档](file-platform-evolution.md)：待实施的能力边界、流程、owner任务与验证入口。

DriveBridge 统一拥有文件、版本、授权、传输与稳定引用，CLI/MCP、Server、Web 是同仓模块；Scaena 仍拥有生成任务。运行入口和限制见 [README](../README.md)。实现与验收记录在 `openspec/changes/archive/`：

- `drivebridge-local-mcp-v1` / `drivebridge-host-handoff-v1` / `drivebridge-remote-client-v1`（2026-09-11 归档）
- 2026-09-11/12 归档：activation-readiness、generic-backends、cli-config（cobra+viper）、agent-workspace（目录+CLI+install）、soft-delete（软删除+恢复，永久删除归用户）

2026-09-18 归档：`drivebridge-storage-onboarding-v1`（kinds/probe/add）、`drivebridge-connection-registry-v1`（命名连接）、`drivebridge-cli-envelope-command-v1`（失败/成功 command 同一）。2026-09-17 归档：creative-relay（创作素材中转）、relay-discovery（中转目标发现）、tenant-context（远程租户上下文）、aigora-client-access（Aigora 客户端访问适配）、local-cas-mount（本机 CAS 缓存 + 只读回环 WebDAV 挂载；FUSE 默认构建返回 `unsupported`）。归档包已含 agent-discovery 批次一 search/changes 与同空间内容寻址 `ReuseUpload`。这属于当时归档记录；当前仍有 Pinax mount change 及新增文件平台设计包，活动状态以本 owner 的 `openspec list` 为准。产品楔子见 [试用操作卡](wedge-trial.md)。

- [首次真实使用 runbook](activation-local.md)
- [本地验证记录](verification.md)
- [远程模式验证](remote-verification.md)
- [创作素材本机中转](creative-resource-relay.md)：Eikona/Scaena/Sonora 本机 relay 已有源码与历史 fixture/race 证据；跨机器真机、分组回存、发布和部署仍独立验收。
- [存储接入 recipe 卡](storage-recipes.md)：storage kinds/probe/add 用法与 gdrive/openlist/s3/nas/local 每类配方。
- [楔子试用操作卡](wedge-trial.md)：个人 Agent 文件往返；setup 完成不算成功。

用量、图表、扫描与安全审计见[使用说明](insights-audit.md)。

- [本机文件、预览与远程交接](local-file-experience.md)
- [本机 CAS 缓存与挂载](local-cas-mount.md)：hash 去重下载、只读回环 WebDAV、远程秒传消费；默认二进制无 FUSE。

## Agent Skills

本项目会话的 active skills 由仓库根 `.skills/profiles/targets/cli/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target cli/drivebridge` 生成 `.agents/skills/` 与 `.claude/skills/`。不要手改运行副本。

完整对照、缺口与下一波优化见 [子项目 Skill Profile](../../../docs/skills/subproject-skill-profiles.md)。

- [持续集成与交付](delivery/ci-cd.md)：按改动选择检查，区分本地、远端与发布证据。

- [语义视频生产交接与本仓任务](semantic-video-production.md)：固定来源、用途、版本、恢复与验收边界。下一阶段的合同、恢复与任务拆分见[归档设计细节](../openspec/changes/archive/2026-10-11-drivebridge-semantic-video-handoff-v1/design.md)。
