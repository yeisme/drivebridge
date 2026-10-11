# aigora-service-access Specification

## Purpose
TBD - created by archiving change drivebridge-aigora-service-access-v1. Update Purpose after archive.
## Requirements
### Requirement: Signed service identity verification

服务 SHALL 用固定 issuer/JWKS 验证 Identity Platform 签发的 EdDSA 服务凭证，audience 固定 `drivebridge`，有效期不超过五分钟，且不得授予上游角色之外的任何管理权。

#### Scenario: Valid short-lived context maps to a local principal

- **WHEN** 未过期的 drivebridge 受众 EdDSA 凭证通过验证且存在精确映射
- **THEN** 请求以映射的既有本地用户身份执行，不获得平台管理员权限

#### Scenario: Wrong audience or expiry rejected

- **WHEN** 凭证受众不是 drivebridge 或签发窗口超过五分钟
- **THEN** 认证拒绝，不产生任何本地授权

### Requirement: Exact identity mapping and cutover

服务 SHALL 仅按 issuer、subject、身份组织的显式三元组建映射并绑定既有本地用户、租户与受限 Agent，切换前所有活跃成员必须已映射，切换后旧凭据不得恢复访问。

#### Scenario: Deployment links an exact identity

- **WHEN** 部署侧命令提供完整 issuer/subject/身份组织/本地用户/租户
- **THEN** 映射生效；同一映射不可改指他人

#### Scenario: Legacy credential after cutover

- **WHEN** 租户已完成切换且请求使用旧用户 Key、会话或 Agent
- **THEN** 服务以 `unified_login_required` 拒绝

### Requirement: Scoped business admission at operation boundaries

商业准入 SHALL 只在业务操作边界执行：托管上传先向 Aigora service-billing 预留用量，未计量、未配置 reporter 凭据或预留被拒的托管写入 SHALL 被拒绝，不得按 HTTP/MCP 请求次数收费。

#### Scenario: Hosted upload reserves before executing

- **WHEN** 统一身份上传携带 entitlement 引用且预留成功
- **THEN** 本地操作与预留台账同事务落库，预留以 reporter 凭据与短期凭证上下文发起

#### Scenario: Unmetered hosted write rejected

- **WHEN** 托管写入缺少 entitlement 引用、reporter 凭据未配置或预留被中心拒绝
- **THEN** 服务分别以 `service_admission_required`、远端拒绝码或 `identity_unavailable` 拒绝，且不落任何本地操作

### Requirement: Usage reporting and recovery reconciliation

服务 SHALL 在业务操作到达明确终态时上报用量（交付=delivered，取消=failed），本地失败保持预留；重启后未上报的预留 SHALL 按本地终态对账，本地记录缺失的按 unknown 结果上报。

#### Scenario: Delivered upload settles its reservation

- **WHEN** 已预留上传完成交付或被取消
- **THEN** 分别以 delivered/实际用量、failed/0 上报，台账转为 reported

#### Scenario: Restart reconciles unreported reservations

- **WHEN** 服务重启时存在未上报预留
- **THEN** 本地 completed 上报 delivered，本地记录缺失上报 unknown_accept，本地 failed 保持持有

### Requirement: Bounded read degradation and key confinement

读取 SHALL 仅在凭证未过期且带 `drivebridge:read` scope 时可用；中心不可用时仅保留降级读取，写入与提权必须在线。SDK SHALL 仅把长期 Key 发送给 Aigora，失败不得回退为长期 Key 直连。

#### Scenario: Outage keeps reads bounded and writes closed

- **WHEN** Identity 中心网络不可用且短期凭证未过期
- **THEN** 读取在 scope 内降级可用，任何托管写入被拒绝

#### Scenario: SDK never leaks the long key to the origin

- **WHEN** Aigora 交换失败或不可用
- **THEN** SDK 不向 DriveBridge 发送长期 Key，也不回退直连

