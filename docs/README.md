# DriveBridge 文档

本项目拥有 OneDrive 文件传输和稳定引用；Scaena 拥有生成任务。运行入口和限制见 [README](../README.md)。实现与验收记录在 `openspec/changes/archive/`：

- `drivebridge-local-mcp-v1` / `drivebridge-host-handoff-v1` / `drivebridge-remote-client-v1`（2026-09-11 归档）
- 2026-09-11/12 归档：activation-readiness、generic-backends、cli-config（cobra+viper）、agent-workspace（目录+CLI+install）、soft-delete（软删除+恢复，永久删除归用户）

2026-09-18 归档：`drivebridge-storage-onboarding-v1`（kinds/probe/add）、`drivebridge-connection-registry-v1`（命名连接）、`drivebridge-cli-envelope-command-v1`（失败/成功 command 同一）。2026-09-17 归档：creative-relay（创作素材中转）、relay-discovery（中转目标发现）、tenant-context（远程租户上下文）、aigora-client-access（Aigora 客户端访问适配）、local-cas-mount（本机 CAS 缓存 + 只读回环 WebDAV 挂载；FUSE 默认构建返回 `unsupported`）。归档包已含 agent-discovery 批次一 search/changes 与同空间内容寻址 `ReuseUpload`。当前无 active change。产品楔子见 [试用操作卡](wedge-trial.md)。

- [首次真实使用 runbook](activation-local.md)
- [本地验证记录](verification.md)
- [远程模式验证](remote-verification.md)
- [创作素材本机中转（设计阶段）](creative-resource-relay.md)：同机与跨机器引用导入、客户端能力判断和回存恢复；新增闭环尚未实现或验证。
- [存储接入 recipe 卡](storage-recipes.md)：storage kinds/probe/add 用法与 gdrive/openlist/s3/nas/local 每类配方。
- [楔子试用操作卡](wedge-trial.md)：个人 Agent 文件往返；setup 完成不算成功。

用量、图表、扫描与安全审计见[使用说明](insights-audit.md)。

- [本机文件、预览与远程交接](local-file-experience.md)
- [本机 CAS 缓存与挂载](local-cas-mount.md)：hash 去重下载、只读回环 WebDAV、远程秒传消费；默认二进制无 FUSE。

## Agent Skills

本项目会话的 active skills 由仓库根 `.skills/profiles/targets/mcp/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target mcp/drivebridge` 生成 `.agents/skills/` 与 `.claude/skills/`。不要手改运行副本。

完整对照、缺口与下一波优化见 [子项目 Skill Profile](../../../docs/skills/subproject-skill-profiles.md)。

