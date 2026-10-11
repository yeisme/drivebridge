# 文件平台公共接口地图与工作台增量

以下表最初用于规划。2026-09-30 源码核对确认其中多组资源已在 `internal/service/http.go` 和 `*_http.go` 注册，具体范围见 [工作台接口基线](../personal-workspace-contracts.md)；不能继续把整张表概括为“全部未实现”，也不能由源码存在推断当前部署已启用。实际调用先读 tools/list、inputSchema、capabilities 和既有 [公共合同](../contract-map.md)，源码/本地验证/真实连接/部署分别报告。新工作台的 grants、收件决定、投影与元数据缺口归 [配套 OpenSpec](../../../openspec/changes/drivebridge-personal-workspace-contracts-v1/README.md)。

| 能力 | 新HTTP资源族 | Agent动作边界 | 规格真源 |
| --- | --- | --- | --- |
| 能力 | capabilities/check | 只读预检，不管理连接 | [readiness](../../../openspec/changes/drivebridge-capability-readiness-v2/design.md) |
| 上传 | upload-batches、operations/lookup | 原批次/原键恢复；字节HTTP | [upload](../../../openspec/changes/drivebridge-upload-batch-recovery-v2/design.md) |
| 云盘 | connections/{id}/entries、changes | 限定root读取，授权配置归人 | [cloud](../../../openspec/changes/drivebridge-cloud-connectors-v2/design.md) |
| 迁移 | transfer-plans | copy执行与源清理分离 | [transfer](../../../openspec/changes/drivebridge-cross-storage-transfer-v1/design.md) |
| 组织 | file-queries、file-collections、saved-searches | 范围内虚拟组织，不改领域状态 | [organization](../../../openspec/changes/drivebridge-file-organization-v1/design.md) |
| 分析 | analytics/queries、analytics/snapshots | 类型化查询，导出需独立权限 | [analytics](../../../openspec/changes/drivebridge-analytics-search-v1/design.md) |
| 内容 | content-bindings、content-queries | 纯查询不触发付费处理 | [content](../../../openspec/changes/drivebridge-content-retrieval-v1/design.md) |
| Agent | organization-plans | 风险/预授权/expected revision | [agent](../../../openspec/changes/drivebridge-agent-organization-v1/design.md) |
| 交付 | shares/{id}/delivery-manifests | 管理权限不授予普通Agent | [delivery](../../../openspec/changes/drivebridge-fixed-delivery-v2/design.md) |
| 分享接收 | shared/{id}/challenge、proof-open、device-open、devices；shares/{id}/receivers | 接收设备为浏览器/本机，不注册Agent工具 | [share-receiver](../../../openspec/changes/drivebridge-share-receiver-v1/design.md) |
| 讨论 | files/{id}/threads、private-note | 当前文件与评论权限 | [discussion](../../../openspec/changes/drivebridge-version-discussion-v1/design.md) |
| 保留 | retention/holds、retention/plans | 不提供Agent purge | [retention](../../../openspec/changes/drivebridge-retention-guards-v2/design.md) |
| 事件 | changes、change-subscriptions、automation-intents | 只消费现有授权，不自批规则 | [automation](../../../openspec/changes/drivebridge-controlled-automation-v1/design.md) |

## 兼容和身份

Server所有新HTTP路径以 `/api/v1/` 开头。Manju路径继续 `/v1/`，不混用资源或Operation ID。旧search、create_upload、complete_upload、operation_get、resume、cancel、usage_*不改名、不加必填字段。新MCP工具与SDK均调用同一service，不shell out CLI。

新写请求按instance/tenant/actor/action隔离幂等键；旧接口保持旧作用域。读操作lookup只能查当前主体，不通过请求actor字段查他人。新的分页默认50/最多200，不透明cursor绑定查询与数据代次；执行计划再以当前版本和权限裁决。

共用错误及各包特殊错误在owner design中维护。错误正文为安全英文静态说明，界面翻译成中文下一步；不能带Provider原文、请求体、token、临时URL或owner内部配置路径。

## 2026-10-09 本地实现补齐

- 分享接收HTTP/SDK与浏览器proof/device流程已本地验收；启用方式及opaque session迁移边界见[媒体分享合同](../media-sharing.md#分享接收与设备绑定显式启用)。新设备能力默认关闭，不向Agent注册工具。
- 普通文件收藏使用 `/favorites`，目录/文件/集合/标签保持完整逻辑身份；删除绑定不删目标。普通Markdown关系使用固定版本 `/files/{id}/links`、`backlinks` 与显式 `links/rebuild`、原键 `links/lookup`；不复制Pinax关系。分页默认50/最多200，候选预算超过时明确拒绝，不伪造全库覆盖。
- 工作区Go SDK已补home/tasks/invocations、grants生命周期、groups/organization、收件/建议、文档、属性与资源检查的类型化消费，复用原HTTP错误/授权/CAS和恢复身份，不自动重试写入。
- 保留保护增加固定文件索引与人类CAS解除，`--hold-management`默认关闭；当前主体、来源与目标固定版本绑定、原回执恢复均由服务端裁决。旧release不能绕过新受管hold的CAS入口。回收站不提供永久删除。

证据分层仍保持：本机fixture/真实云/原生系统/发布部署互不替代。桌面受管vault的旧writer准入前提未满足，不能据上述普通文件功能宣称Pinax绑定可用。
