# Proposal

## Why

DriveBridge 需要接入 Aigora 统一服务身份与商业准入：短期 EdDSA 服务凭证替代长期用户 Key 直连，托管（统一身份）写入必须经过业务操作级计量，且租户切换后旧凭据不得继续访问。此前托管写入一律 fail-closed（`service_admission_required`），没有任何计量与对账路径。

## What Changes

- 验证 Identity Platform 签发的 EdDSA 服务凭证：audience 固定 `drivebridge`、有效期最长五分钟、issuer/JWKS/Identity backend 固定。
- `ServiceIdentityBinding` 精确映射（issuer+subject+身份组织 → 既有本地用户/租户/受限 Agent），不按邮箱合并；`governance identity-link` 部署侧命令维护；cutover 前检查全部活跃成员已映射，切换后旧用户 Key/会话/Agent 被拒绝（`unified_login_required`）。
- 托管读取在凭证未过期且带 `drivebridge:read` scope 时可用，中心不可用仅限降级读取；写入与提权必须在线核验。
- 业务操作级准入：`ServiceAdmission` 台账 + Aigora `service-billing reserve/report`（reporter credential，配置 `service_access_reporter_credential_file`）；上传边界先预留再落本地状态，交付/取消即时上报，重启后未上报预留做恢复对账（delivered / failed / unknown_accept，本地 failed 保持持有）；未计量或未配置的托管写入一律拒绝。
- SDK `NewAigora` 仅把长期 Key 送至 Aigora、线程安全缓存短期凭证、失败不回退；MCP 适配器 `aigora_url`（mcp/drivebridge owner）。

## Impact

- 新增表：`ServiceIdentityBinding`、`ServiceAdmission`（仅追加迁移）。
- 新增配置：`service_identity_issuer`、`service_identity_endpoint`、`service_access_reporter_credential_file`（默认关闭）。
- 新增错误码：`unified_login_required`、`identity_mapping_required`、`identity_unavailable`、`service_admission_required`、`insufficient_funds`、`cross_tenant_admin_context_required`。
- 受影响面：`internal/service`（身份验证、准入、上传生命周期）、`sdk`、`cmd/drivebridge-server`、`internal/config`。
