## ADDED Requirements

### Requirement: Scoped key directory

服务 SHALL 仅在调用者凭据管理范围内返回 Key 的公开元数据，不暴露明文或 token hash。

#### Scenario: Governance administrator reads key metadata

- **WHEN** 治理管理员请求 Key 目录
- **THEN** 无论选中的数据空间如何，仅返回该认证用户本人的 Key

### Requirement: Identity-preserving key labels

服务 SHALL 支持修改非归档 Key 的标签，且不改变所属用户、凭据或权限。

#### Scenario: User renames their key

- **WHEN** 用户为本人的 Key 提供有效标签
- **THEN** 标签更新，所属用户和凭据保持一致

#### Scenario: User targets another user's key

- **WHEN** 普通用户请求修改他人的 Key 标签
- **THEN** 服务拒绝该修改
