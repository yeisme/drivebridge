## ADDED Requirements

### Requirement: Running-deployment key rotation
admin SHALL 可经 HTTP 轮换用户密钥（运行中部署无需停服窗口）；已吊销或已过期的密钥不可轮换。

#### Scenario: Admin rotates over HTTP
- **WHEN** admin 调用密钥轮换接口
- **THEN** 新密钥一次性返回，旧密钥立即失效，吊销级联语义同 CLI

#### Scenario: Non-admin rejected
- **WHEN** 非 admin 凭据调用轮换接口
- **THEN** 返回 forbidden

#### Scenario: Expired key not rotatable
- **WHEN** 对已过期或已吊销的密钥发起轮换
- **THEN** 返回 not_found，不产生出生即死的新密钥
