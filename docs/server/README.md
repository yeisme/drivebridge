# 文档索引

- [个人多 Agent 工作台接口合同](personal-workspace-contracts.md)：与当前 OpenDesign 对齐的聚合/任务、窄授权、受限收件和文档资源增量设计；新增路径待实施。
- [工作台数据模型与兼容迁移](personal-workspace-data.md)：GORM、固定版本、授权修订、幂等、发布前撤权和旧版本降级保护。
- [工作台后端 OpenSpec](../../openspec/changes/drivebridge-personal-workspace-contracts-v1/README.md) / [前端 PRD](../web/product/personal-agent-workspace-prd.md)：正式任务与产品验收入口；文件领域逻辑仍由 Server 拥有。

- [文件平台数据分层与实施规格](capability-evolution.md)：14项能力、配套owner包及上传/云端/分析/内容/Agent设计；规模与真实后端均待验收。
- [后续能力设计](capability-evolution.md)：基于现有实现补齐原操作恢复、固定交付、版本讨论和持续同步；提案阶段，包含归属边界、场景矩阵与验收条件。
- [运行与接口](../../server/README.md)
- 本地实现与验收：`openspec/changes/archive/`（`drivebridge-personal-service-v1`、`drivebridge-deployment-readiness-v1`，2026-09-11 归档）
- Platform v2 三 change 已实现并归档（2026-09-12）：`drivebridge-tenant-isolation-v2`/`drivebridge-backup-restore-v2`/`drivebridge-content-encryption-v1`；`drivebridge-server-config-v1`（2026-09-12 归档：cobra 命令树+viper 部署配置文件）
- `drivebridge-tenant-shared-storage-v1`（2026-09-14 归档：COS 虚拟主机寻址 + 共享桶租户命名空间 + `platform-connect --verify-isolation` 隔离探针，真实腾讯云 COS 验收）
- `drivebridge-tenant-key-users-v1`（2026-09-14 归档：操作员批量签发 `dbu_` 用户密钥 + key-only serve（无 issuer 可启动）+ admin 密钥 HTTP 开面 + 新租户四步配方）
- `drivebridge-users-rotate-endpoint-v1`（2026-09-16 归档：review 三修——ListAgents 列名/RotateUserKey 过期守卫/HTTP rotate 端点）
- `drivebridge-session-revocation-cascade-v1`（2026-09-15 归档：吊销/轮换级联杀换发会话，OIDC 会话不受影响）
- `drivebridge-user-external-keys-v1`（2026-09-15 归档：外部预生成 key 导入（32–128 可打印任意前缀）+ 哈希优先路由 + `users rotate` 原子轮换 + 去 LiteLLM）
- `drivebridge-user-batch-csv-v1`（2026-09-15 归档：`users import --csv/--out` 填表批量签发 + `scripts/issue_users.py` 可选 LiteLLM 列）
- `drivebridge-key-session-login-v1`（2026-09-14 归档：`POST /auth/key` 密钥换发 cookie 会话 + admin 会话继承 + users list/revoke HTTP + `/auth/mode`；并行 lane `drivebridge-users-agent-surface-v1` MCP users 工具随波合并）

- [公共合同与 owner](contract-map.md)
- [内网 CAS 与 blob 复用](intranet-cas.md)：明文 SHA-256 去重、秒传、`blob stat`/`gc` 与备份交叉已实现（`drivebridge-cas-blob-store-v1`）；真实存储验收另报。
- [创作项目文件交接与回存](creative-resource-handoff.md)：接收绑定、固定版本读取与 Eikona 恢复已有源码和历史本地证据；其他消费者完整闭环、真实后端和部署分开验收。
- [生产部署 runbook](deployment.md)
- [本轮验证记录](verification.md)

用量、图表、扫描与安全审计见[使用说明](insights-audit.md)。

- [多租户、团队与管理授权](tenant-governance.md)

- [媒体预览、分享与创作交接](media-sharing.md)

- [团队工作区合同](team-workspace.md)：生命周期、邀请、存储分配、归档、版本冲突和本地团队体验。

- [登录 Key 管理](key-management.md)：本人复制、再次取回、轮转及兼容迁移。

- [用户目录与账号管理](user-management.md)

- 2026-09-17 归档：`drivebridge-creative-handoff-v1`（创作交接 HTTP/MCP/CLI 闭环）、`drivebridge-handoff-interaction-v1`（交接交互细节；独立 v0.8.1 候选保持冻结的决策保留在归档）、`drivebridge-key-directory-v1`（用户目录与 key 管理；Web 浏览器验证由 client owner 单独报告）、`drivebridge-multitenant-governance-v1`（治理启用、租户成员、恢复流程与 `tenant-governance` 新规格）。

2026-09-17 归档（续）：`drivebridge-aigora-service-access-v1`（13/13，真实 Identity Platform/Aigora 联调需单独授权）、`drivebridge-cas-blob-store-v1`（11/11，真实 COS/内网磁盘验收另报）。`drivebridge-key-delete-trash-batch-v1` 后已归档；当前新增文件平台设计包见上方导航，任务状态以各 owner 的 `openspec list` 为准。

## Agent Skills

本项目会话的 active skills 由仓库根 `.skills/profiles/targets/cli/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target cli/drivebridge` 生成 `.agents/skills/` 与 `.claude/skills/`。不要手改运行副本。

完整对照、缺口与下一波优化见 [子项目 Skill Profile](../../../../docs/skills/subproject-skill-profiles.md)。


- [本地团队体验恢复](demo-team-recovery.md)
