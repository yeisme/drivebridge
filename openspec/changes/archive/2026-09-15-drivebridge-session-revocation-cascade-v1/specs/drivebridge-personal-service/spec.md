## ADDED Requirements

### Requirement: Session revocation cascade
密钥换发会话 SHALL 跟随来源密钥生命周期：来源密钥被吊销或过期后，会话下一次认证即失效；非密钥来源会话不受影响。

#### Scenario: Revoked key kills its sessions
- **WHEN** 用户密钥被吊销或轮换后，其换发会话发起请求
- **THEN** 返回 unauthorized，会话立即失效

#### Scenario: OIDC sessions unaffected
- **WHEN** 任一用户密钥被吊销
- **THEN** OIDC 登录建立的会话不受影响
