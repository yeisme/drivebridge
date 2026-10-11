# 公共合同与 owner

HTTP 路由：`internal/service/http.go`。身份和资源判断：`auth.go`、`model.go`。上传与版本：`files.go`。连接：`storage.go`、`onedrive.go`。MCP registry：`tools.go`，适配器：`mcp.go`。共享 Go 客户端：`sdk/client.go`。

新增远程引用：`drivebridge://<instance-id>/file/<file-id>/version/<version-id>`。实例 ID 随元数据备份恢复；不把旧本机引用自动迁入。临时 S3 URL 不作为文件地址持久化。

内网 CAS（已实现）：内容身份是明文 SHA-256 blob（`blobs` 表 + `versions.blob_sha256`），明文对象键 `<instance>/blobs/<sha256>`、加密 `<instance>/blobs/<space>/<sha256>`；秒传命中返回 `blob_reused`，`GET /api/v1/blobs/{sha256}` 越权 `not_found`，漂移报 `blob_unavailable`。旧 file/version 引用保持。见[内网 CAS](intranet-cas.md)与根设计 `openspec/changes/archive/2026-09-17-drivebridge-intranet-cas-mount-v1/`。

本服务不保存生成任务、提示词日志、Provider 收费状态或领域 canonical 审核结果。Scaena 的准入与生成回执保留原 owner。

## 创作资源复用的增量设计

Scaena、Eikona、Sonora 的项目内引用、固定副本导入与选定交付版本回存见[交接合同与分层证据](creative-resource-handoff.md)。接收绑定、固定版本读取及 Eikona 所需恢复已有源码与历史 fixture/race 记录；其他消费者完整闭环和真实部署独立验收。工具注册与就绪标签不能作为跨项目回存完整的证明。2026-09-25 新增[文件平台规划接口地图](file-platform/interfaces.md)，只完善规格和文档，不改变当前公共合同或旧 URI。

## 动作能力就绪与团队运行时（已实现，本地验证）

能力预检（`GET/POST /api/v1/capabilities*`、MCP `capability_check`）、团队运行时（`/api/v1/runtime/status`、投影代次重建、`metadata export/import/verify` 迁移 CLI）的公开合同、真实命令与回滚演练见[能力就绪与运行时状态](file-platform/runtime-status.md)。观测与配置分离、fencing 租约、事务性 Outbox 与静默迁移均已落地并由 `TestFPReadiness*`/`TestFPTeamRuntime*` 本地验证；真实后端与发布独立验收。

## 版本讨论与私人备注（已实现，本地验证）

版本线程与并发：`files/{id}/threads` 列表/创建（幂等）、`threads/{id}/replies|resolve|reopen`、`thread-messages/{id}` 编辑/删除（expected_revision）、`files/{id}/private-note` 读写与 `publish-preview`；MCP/SDK 同名入口共用应用服务。详见[内容交付与讨论状态](file-platform/content-delivery.md)。

## 版本保护与保留计划（已实现，本地验证）

保留保护与清理互斥：`retention/holds` 创建/读取/解除（幂等键同键同指纹）、`retention/plans` 预览与人工执行（无 Agent purge 工具）；purge 与 hold 在同一事务真源上裁决（`version_protected`/`purge_in_progress`/`retention_plan_stale`）。详见[内容交付与保留状态](file-platform/content-delivery.md)。

## 上传批次与丢失响应恢复（已实现，本地验证）

批次组织原 Operation：`upload-batches` 创建/分页登记/seal/游标读取/控制，`operations/lookup` 以同键同 `original_request` 找回原身份，`operations/{id}/session` 回读耐久分片会话；MCP/SDK 同名入口共用应用服务。详见[上传与恢复状态](file-platform/uploads-cloud.md)。

## 空间配额字段

`/api/v1/spaces`、MCP `spaces` 与 `drivebridge-server status` 共用同一配额口径：每空间 `max_bytes`（上限）、`used_bytes`（已提交占用）、`reserved_bytes`（在途操作预留），可用余量 = `max_bytes - used_bytes - reserved_bytes`；配额在操作预留阶段即扣减（`quota_exceeded` 早拒绝），不等到字节写完。

## 多租户治理

已显式启用治理的部署使用租户成员关系与独立空间归属；未启用部署保留原单租户合同。治理、审批、恢复和上下文接口见 [多租户治理](tenant-governance.md)。真实 actor 与存储 owner 分离；旧 admin 标记不自动扩权。

## Aigora 统一身份与业务准入

配置默认关闭：`service_identity_issuer`（固定 issuer，附带 JWKS/Identity backend）、`service_identity_endpoint`（可选显式后端）、`service_access_reporter_credential_file`（托管写入计量 reporter 凭据文件，缺失即 fail-closed）。短期 EdDSA 凭证 audience 固定 `drivebridge`、有效期≤5 分钟；`governance identity-link` 维护 issuer/subject/身份组织到既有本地用户/租户/受限 Agent 的精确映射，cutover 后旧凭据以 `unified_login_required` 拒绝。托管读取需 `drivebridge:read` scope，中心不可用仅限降级读取；托管写入需在线 + `drivebridge:write` scope，且只在业务操作边界计量（上传先向 service-billing `reserve` 后落库，交付/取消即 `report`，重启对未上报预留按本地终态对账），未计量写入一律 `service_admission_required` 拒绝。SDK `NewAigora` 仅把长期 Key 送 Aigora，失败不回退直连。

能力发现：`GET /api/v1/capabilities` 与 MCP `capabilities` 工具均在 `service_identity` 字段报告统一身份状态（`configured`、`cutover`、`aigora_url` 为 `supported`/`blocked`、`block_reason`、`error_codes`）。MCP 适配器（cli/drivebridge owner）仅在该字段为 `supported` 时提供 `aigora_url` 连接形态；未配置时必须向客户端展示阻塞原因，不得假装可用，客户端无需安装 CLI。

统一身份与准入的六个错误码在 HTTP 信封（`{"error":{"code","message"}}`，message 为静态英文、不插值请求内容）中形状稳定，Web 与 MCP 客户端按 code 分支：

| code | HTTP | 含义 |
| --- | --- | --- |
| `unified_login_required` | 403 | 租户已切换，旧凭据不再可用，需统一登录 |
| `identity_mapping_required` | 403 | 缺少 issuer/subject/身份组织精确映射 |
| `identity_unavailable` | 503 | 身份中心不可用，写入与提权暂停 |
| `service_admission_required` | 503 | 托管操作未接通计量准入，fail-closed |
| `insufficient_funds` | 409 | 服务钱包不足以覆盖预留 |
| `cross_tenant_admin_context_required` | 403 | 跨租户管理上下文需单独授权 |
