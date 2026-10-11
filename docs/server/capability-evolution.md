# DriveBridge 后续能力与实施规格导航

2026-09-25：本文件从早期讨论稿收敛为owner规格导航。十四项必需能力与配套包已经完成设计拆分；产品代码、运行时验证、真实云/模型与部署均不由本轮证明。

## 已确认方向

- 百万文件、千万事件、万文件批次与100GiB文件为待验收目标。
- OneDrive/S3/Google Drive服务端直管；保留本机与各领域local/S3/Capsa入口。
- SQLite单机、PostgreSQL团队事务、ClickHouse在线分析、DuckDB受控快照；Manju语义检索。
- 总览＋文件详情，图表可钻取；Agent风险分级，虚拟整理优先。
- 原操作恢复、固定交付、版本讨论、同步保留和受限自动化全部保留。

## 本Server实施规格

| 能力包 | 唯一owner | 依赖 | 状态 |
| --- | --- | --- | --- |
| [drivebridge-capability-readiness-v2](../../openspec/changes/drivebridge-capability-readiness-v2/README.md) | `cli/drivebridge/server` | 既有文件合同 | required / design；实施任务未完成 |
| [drivebridge-team-runtime-projections-v1](../../openspec/changes/drivebridge-team-runtime-projections-v1/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2` | required / design；实施任务未完成 |
| [drivebridge-upload-batch-recovery-v2](../../openspec/changes/drivebridge-upload-batch-recovery-v2/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-cloud-connectors-v2](../../openspec/changes/drivebridge-cloud-connectors-v2/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-cross-storage-transfer-v1](../../openspec/changes/drivebridge-cross-storage-transfer-v1/README.md) | `cli/drivebridge/server` | `drivebridge-cloud-connectors-v2`、`drivebridge-upload-batch-recovery-v2` | required / design；实施任务未完成 |
| [drivebridge-file-organization-v1](../../openspec/changes/drivebridge-file-organization-v1/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-analytics-search-v1](../../openspec/changes/drivebridge-analytics-search-v1/README.md) | `cli/drivebridge/server` | `drivebridge-team-runtime-projections-v1`、`drivebridge-file-organization-v1` | required / design；实施任务未完成 |
| [drivebridge-content-retrieval-v1](../../openspec/changes/drivebridge-content-retrieval-v1/README.md) | `cli/drivebridge/server` | `drivebridge-file-organization-v1`、`drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-agent-organization-v1](../../openspec/changes/drivebridge-agent-organization-v1/README.md) | `cli/drivebridge/server` | `drivebridge-file-organization-v1`、`drivebridge-cross-storage-transfer-v1`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-fixed-delivery-v2](../../openspec/changes/drivebridge-fixed-delivery-v2/README.md) | `cli/drivebridge/server` | `drivebridge-file-organization-v1`、`drivebridge-upload-batch-recovery-v2`、`drivebridge-retention-guards-v2` | required / design；实施任务未完成 |
| [drivebridge-version-discussion-v1](../../openspec/changes/drivebridge-version-discussion-v1/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-controlled-automation-v1](../../openspec/changes/drivebridge-controlled-automation-v1/README.md) | `cli/drivebridge/server` | `drivebridge-team-runtime-projections-v1`、`drivebridge-agent-organization-v1`、`drivebridge-content-retrieval-v1` | required / design；实施任务未完成 |
| [drivebridge-retention-guards-v2](../../openspec/changes/drivebridge-retention-guards-v2/README.md) | `cli/drivebridge/server` | `drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |

## 本地文档

- [架构与数据分层](file-platform/architecture.md)
- [接口地图](file-platform/interfaces.md)
- [上传与云端管理](file-platform/uploads-cloud.md)
- [查询分析与Agent整理](file-platform/query-organization.md)
- [内容交接、交付和讨论](file-platform/content-delivery.md)
- [迁移与容量验收](file-platform/verification.md)

## 路线与状态

先稳定readiness/team运行，再推进上传/云连接/虚拟组织；随后跨存储、分析、内容和Agent计划；Client随稳定合同消费，交付/讨论独立增量；同步与自动化使用已验证恢复基础。详细跨owner依赖由根handoff导航，本owner任务不由根包代写完成。

当前已有上传分片、固定引用、CAS、分享、团队权限和交接部分代码；不能把“新设计待实现”理解为这些旧能力不存在。旧文档的fixture证据保留；本轮未重新执行。部署和真实后端状态必须从实际环境验证。

任何新路径、工具和配置以owner设计中的规划标签为准；安装客户端仍先发现真实能力。独立checkout无需根文档即可从上表进入本owner设计、任务和验证。
