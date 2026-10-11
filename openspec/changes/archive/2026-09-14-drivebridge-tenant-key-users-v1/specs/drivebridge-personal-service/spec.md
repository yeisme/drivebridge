## ADDED Requirements

### Requirement: Operator-issued user keys
系统 SHALL 支持操作员在本地终端批量签发或导入用户密钥以创建用户身份：密钥明文仅在签发输出出现一次，存储只保留哈希，生命周期可查询与吊销；用户密钥身份不经 OAuth/OIDC 自助注册。

#### Scenario: Batch import generates keys
- **WHEN** 操作员经 CLI 提交 N 条不带密钥的用户条目
- **THEN** 系统为每条生成 `dbu_` 格式密钥并创建用户，明文密钥只在本次签发输出出现一次

#### Scenario: Operator-supplied keys
- **WHEN** 条目携带操作员自备密钥
- **THEN** 系统校验格式与唯一性后导入，格式不符或哈希重复时整批拒绝且不落任何新用户

#### Scenario: Revoked key rejected
- **WHEN** 已吊销或已过期的用户密钥发起认证
- **THEN** 返回 unauthorized，不区分"不存在"与"已吊销"的响应体

### Requirement: Key-only deployment
部署 SHALL 支持无 issuer 的 key-only 模式：`serve --enabled` 不因缺少 OIDC 配置拒绝启动，浏览器登录如实报告 oidc_not_configured，Bearer 用户密钥照常认证。

#### Scenario: Boot without issuer
- **WHEN** 以 `--enabled` 启动且未配置 issuer/client-id
- **THEN** 服务正常监听，密钥 Bearer 请求按正常权限处理

#### Scenario: Browser login honestly rejected
- **WHEN** key-only 部署下访问浏览器登录入口
- **THEN** 返回 oidc_not_configured，不降级为其他登录方式

### Requirement: Admin-key user provisioning over HTTP
持有 admin 用户密钥的调用方 SHALL 可经 HTTP 批量创建用户，校验与操作员 CLI 一致；普通用户密钥、Agent 令牌与会话 SHALL 被拒绝且不泄露存在性。

#### Scenario: Admin key creates batch
- **WHEN** admin 用户密钥调用用户创建接口提交批量条目
- **THEN** 与 CLI 同规则创建用户并一次性返回明文密钥

#### Scenario: Non-admin rejected
- **WHEN** 普通用户密钥、Agent 令牌或浏览器会话调用用户创建接口
- **THEN** 返回 forbidden，不创建任何用户
