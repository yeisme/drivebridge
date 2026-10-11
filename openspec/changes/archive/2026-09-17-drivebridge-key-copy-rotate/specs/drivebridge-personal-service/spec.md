## MODIFIED Requirements

### Requirement: Operator-issued user keys
系统 SHALL 支持操作员在本地终端批量签发或导入用户密钥以创建用户身份：密钥明文仅在签发输出出现一次，认证保留哈希，并另存绑定用户与 Key ID 的加密密文供本人再次取回，生命周期可查询与吊销；用户密钥身份不经 OAuth/OIDC 自助注册。重复哈希 SHALL 以 `user_key_duplicate` 报告且整批不落库；数据库失败 SHALL 以 `database_unavailable` 报告，不共用同一码。

#### Scenario: Batch import generates keys
- **WHEN** 操作员经 CLI 提交 N 条不带密钥的用户条目
- **THEN** 系统为每条生成 `dbu_` 格式密钥并创建用户，明文密钥只在本次签发输出出现一次

#### Scenario: Operator-supplied keys
- **WHEN** 条目携带操作员自备密钥
- **THEN** 系统校验格式与唯一性后导入，格式不符或哈希重复时整批拒绝且不落任何新用户

#### Scenario: Revoked key rejected
- **WHEN** 已吊销或已过期的用户密钥发起认证
- **THEN** 返回 unauthorized，不区分"不存在"与"已吊销"的响应体

### Requirement: Session revocation cascade
密钥换发会话 SHALL 跟随来源密钥生命周期：来源密钥被吊销或过期后，会话下一次认证即失效；非密钥来源会话不受影响。本人通过专用接口确认轮转时，SHALL 仅将当前操作会话的来源迁移至新 Key。

#### Scenario: Revoked key kills its sessions
- **WHEN** 用户密钥被吊销或轮换后，除本人轮转时已迁移的当前会话外，其余换发会话发起请求
- **THEN** 返回 unauthorized，会话立即失效

#### Scenario: OIDC sessions unaffected
- **WHEN** 任一用户密钥被吊销
- **THEN** OIDC 登录建立的会话不受影响

