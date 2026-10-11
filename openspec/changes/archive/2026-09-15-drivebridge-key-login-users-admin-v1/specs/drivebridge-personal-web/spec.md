## ADDED Requirements

### Requirement: Dual-path login with key exchange
登录页 SHALL 按 `/auth/mode` 如实呈现登录路径：OIDC 可用时提供账号登录，密钥登录作为输入路径始终可用；密钥换发会话后浏览器不保留密钥。

#### Scenario: Key-only deployment
- **WHEN** 部署未配置 OIDC 且用户在登录页
- **THEN** 仅呈现密钥登录输入，无 OIDC 按钮

#### Scenario: Wrong key
- **WHEN** 提交无效密钥
- **THEN** 显示登录失败提示，不区分不存在与已吊销

#### Scenario: No key persisted client-side
- **WHEN** 密钥登录成功后检查浏览器存储
- **THEN** 密钥不出现在 localStorage、sessionStorage 或 URL

### Requirement: Admin key-user management page
admin 用户 SHALL 在网页端管理用户密钥：查看生命周期列表、批量签发（名称/天数/admin）、一次性展示明文密钥、吊销；非 admin 不可见此入口，且服务端仍是权威鉴权。

#### Scenario: Admin issues users
- **WHEN** admin 提交批量签发表单
- **THEN** 明文密钥在一次性弹窗展示并可复制，关闭后不可再取，列表即时更新

#### Scenario: One-time display
- **WHEN** 签发弹窗关闭后再次进入管理页
- **THEN** 列表只有生命周期元数据，无任何明文或哈希

#### Scenario: Revoke with confirmation
- **WHEN** admin 吊销某密钥并确认
- **THEN** 该行翻转为已吊销状态

#### Scenario: Hidden from non-admin
- **WHEN** 普通用户登录
- **THEN** 导航不出现用户密钥入口
