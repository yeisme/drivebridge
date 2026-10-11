## ADDED Requirements

### Requirement: External pre-generated key import
系统 SHALL 支持导入外部预生成密钥：去空白后长度 32–128 的可打印 ASCII 均可导入（任意前缀）；与既有用户密钥或 Agent 令牌哈希冲突时整批拒绝零落库；自生成密钥保持 dbu_ 格式不变。

#### Scenario: Foreign-format key imported
- **WHEN** 操作员导入外部工具生成的密钥（非 dbu_ 前缀、可打印且长度达标）
- **THEN** 导入成功，该密钥经 Bearer 与密钥登录均按用户身份认证

#### Scenario: Entropy floor enforced
- **WHEN** 导入短于 32 字符、含空白或超过 128 字符的密钥
- **THEN** 整批拒绝且不落任何新用户

#### Scenario: Cross-table collision rejected
- **WHEN** 导入密钥的哈希与任一 Agent 令牌或既有用户密钥相同
- **THEN** 整批拒绝

### Requirement: Key rotation
系统 SHALL 提供密钥原子换发：同一用户铸新密钥并吊销旧密钥，明文只在本次输出出现一次，旧密钥下一次认证即被拒。

#### Scenario: Rotate keeps identity
- **WHEN** 管理员轮换某用户的密钥
- **THEN** 新密钥立即以同一用户身份工作，旧密钥立即失效，用户与其数据归属不变
