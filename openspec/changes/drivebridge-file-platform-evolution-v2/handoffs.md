# 能力账本与owner交接

根包维护路由，不追踪子项目实现勾选。以下所有新包均为required/design；实际源码、本地验证、真实后端与发布状态在owner内独立记录。

## 15个主能力包

| 能力包 | 唯一owner | 依赖 | 状态 |
| --- | --- | --- | --- |
| [drivebridge-capability-readiness-v2](../drivebridge-capability-readiness-v2/README.md) | `cli/drivebridge/server` | 既有文件合同 | required / design；实施任务未完成 |
| [drivebridge-team-runtime-projections-v1](../drivebridge-team-runtime-projections-v1/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2` | required / design；实施任务未完成 |
| [drivebridge-upload-batch-recovery-v2](../drivebridge-upload-batch-recovery-v2/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-cloud-connectors-v2](../drivebridge-cloud-connectors-v2/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-cross-storage-transfer-v1](../drivebridge-cross-storage-transfer-v1/README.md) | `cli/drivebridge/server` | `drivebridge-cloud-connectors-v2`、`drivebridge-upload-batch-recovery-v2` | required / design；实施任务未完成 |
| [drivebridge-file-organization-v1](../drivebridge-file-organization-v1/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-analytics-search-v1](../drivebridge-analytics-search-v1/README.md) | `cli/drivebridge/server` | `drivebridge-team-runtime-projections-v1`、`drivebridge-file-organization-v1` | required / design；实施任务未完成 |
| [drivebridge-content-retrieval-v1](../drivebridge-content-retrieval-v1/README.md) | `cli/drivebridge/server` | `drivebridge-file-organization-v1`、`drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-agent-organization-v1](../drivebridge-agent-organization-v1/README.md) | `cli/drivebridge/server` | `drivebridge-file-organization-v1`、`drivebridge-cross-storage-transfer-v1`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-information-workspace-v2](../drivebridge-information-workspace-v2/README.md) | `cli/drivebridge/web` | `drivebridge-capability-readiness-v2`、`drivebridge-analytics-search-v1`、`drivebridge-file-organization-v1` | required / design；实施任务未完成 |
| [drivebridge-fixed-delivery-v2](../drivebridge-fixed-delivery-v2/README.md) | `cli/drivebridge/server` | `drivebridge-file-organization-v1`、`drivebridge-upload-batch-recovery-v2`、`drivebridge-retention-guards-v2` | required / design；实施任务未完成 |
| [drivebridge-version-discussion-v1](../drivebridge-version-discussion-v1/README.md) | `cli/drivebridge/server` | `drivebridge-capability-readiness-v2`、`drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-sync-retention-v2](../drivebridge-sync-retention-v2/README.md) | `cli/drivebridge` | `drivebridge-capability-readiness-v2`、`drivebridge-retention-guards-v2` | required / design；实施任务未完成 |
| [drivebridge-controlled-automation-v1](../drivebridge-controlled-automation-v1/README.md) | `cli/drivebridge/server` | `drivebridge-team-runtime-projections-v1`、`drivebridge-agent-organization-v1`、`drivebridge-content-retrieval-v1` | required / design；实施任务未完成 |
| [drivebridge-share-receiver-v1](../drivebridge-share-receiver-v1/README.md) | `cli/drivebridge/server` | `drivebridge-fixed-delivery-v2` | required / design；实施任务未完成 |

## 4个配套owner包

| 能力包 | 唯一owner | 依赖 | 状态 |
| --- | --- | --- | --- |
| [drivebridge-retention-guards-v2](../drivebridge-retention-guards-v2/README.md) | `cli/drivebridge/server` | `drivebridge-team-runtime-projections-v1` | required / design；实施任务未完成 |
| [drivebridge-remote-surfaces-v2](../drivebridge-remote-surfaces-v2/README.md) | `cli/drivebridge` | `drivebridge-upload-batch-recovery-v2`、`drivebridge-cloud-connectors-v2`、`drivebridge-analytics-search-v1`、`drivebridge-agent-organization-v1`、`drivebridge-controlled-automation-v1` | required / design；实施任务未完成 |
| [drivebridge-file-platform-consumers-v2](../drivebridge-file-platform-consumers-v2/README.md) | `cli/drivebridge/web` | `drivebridge-information-workspace-v2`、`drivebridge-upload-batch-recovery-v2`、`drivebridge-cross-storage-transfer-v1`、`drivebridge-agent-organization-v1`、`drivebridge-content-retrieval-v1`、`drivebridge-fixed-delivery-v2`、`drivebridge-share-receiver-v1`、`drivebridge-version-discussion-v1` | required / design；实施任务未完成 |
| [manju-drivebridge-file-platform-integration-v1](../../../../../backend-server/manju/openspec/changes/manju-drivebridge-file-platform-integration-v1/README.md) | `backend-server/manju` | `drivebridge-content-retrieval-v1` | required / design；实施任务未完成 |

## 跨owner冻结交接

| producer → consumer | 固定合同 | 权限与恢复 | 不转移的状态 |
| --- | --- | --- | --- |
| Server → 本机/Client | capability、batch、operation、query、plan、manifest的公开HTTP/SDK | 当前身份/范围、原ID回读；缺能力明确unsupported | Server元数据、授权与云凭据 |
| 本机 → Server/领域项目 | 固定ref、source digest、target binding与原relay | 只读允许root；字节走受控HTTP；source改变拒绝 | 本机路径、挂载与本地凭据 |
| DriveBridge → Manju | instance/file/version/sha256、已配置receiver binding与purpose | 双边身份映射、范围交集、原grant/import对账 | 资产、理解、索引、主体与领域审核 |
| Anatomia/Sonora → Manju | producer revision、source digest、locator、truth、coverage | 当前producer权限；缺模态明确missing/unsupported | 视频时序或音频领域状态 |
| 事务 → ClickHouse/DuckDB | event ID、revision、generation、watermark、tombstone、授权范围 | 聚合前权限检查；不透明游标；快照明确导出 | 事务与权限真源 |

## Manju既有依赖

新联调包只增加DriveBridge绑定、producer adapter消费和跨owner验证。Inbox、Understanding、Evidence、Retrieval、Visual、Collections、Delivery、OIDC原包以及Anatomia既有handoff不重新初始化、不改旧任务完成状态。

## 任务规则

各包采用六组：合同基线、模型与迁移、应用服务、消费入口、验证、文档与回滚。每项列owner、依赖、范围、断言、命令、证据和失败回查；实际源码/fixture由同一owner维护。依赖既有任务时引用其包与任务号，不将其复制为本包完成项。

纯MCP用户不需要安装CLI；配置及挂载等owner动作明确执行主机。任何SDK/adapter不得shell out产品CLI、读取其他owner私有数据库或复制Provider状态。部署、发布与真实服务启用不在本次交接执行范围。
