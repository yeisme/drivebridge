## ADDED Requirements

### Requirement: Key exchange session login
key-only 部署 SHALL 提供密钥换发会话登录：用户提交一次用户密钥换取与 OIDC 同构的 cookie 会话；浏览器不存储密钥，登出或过期即失效。

#### Scenario: Valid key exchanged for session
- **WHEN** 有效用户密钥提交到密钥登录入口
- **THEN** 服务设置会话 cookie 并可像 OIDC 会话一样访问全部用户接口

#### Scenario: Invalid or revoked key
- **WHEN** 提交错误、已吊销或已过期的密钥
- **THEN** 返回 unauthorized，不区分不存在与已吊销

#### Scenario: No token persists in browser
- **WHEN** 密钥换发会话完成后检查浏览器存储
- **THEN** 密钥不出现在任何客户端存储，仅存在服务端会话 cookie

### Requirement: Admin session inheritance
admin 用户密钥换发的会话 SHALL 继承 admin 属性；管理判定统一由 Principal 承载（密钥 Bearer 或 admin 会话均可），非 admin 会话与凭据不得触达管理接口。

#### Scenario: Admin session reaches management surface
- **WHEN** admin 密钥换发的会话调用用户管理接口
- **THEN** 与 admin 密钥 Bearer 同权限

#### Scenario: Plain session rejected
- **WHEN** 非 admin 会话或非 admin 密钥调用用户管理接口
- **THEN** 返回 forbidden

### Requirement: User management HTTP surface
系统 SHALL 提供 HTTP 用户管理面（列表与吊销），仅 admin 可用；列表只含生命周期元数据，不含哈希或明文密钥。

#### Scenario: Admin lists users
- **WHEN** admin 调用用户列表接口
- **THEN** 返回全部用户密钥的生命周期元数据（id/用户/名称/admin/过期/吊销）

#### Scenario: Admin revokes a key
- **WHEN** admin 调用吊销接口指定密钥 id
- **THEN** 该密钥下一次认证被拒，接口返回成功

### Requirement: Honest login mode reporting
系统 SHALL 如实报告本部署可用的登录方式（OIDC 与/或密钥），该报告无需认证且不泄露租户敏感信息。

#### Scenario: Key-only deployment
- **WHEN** 未配置 issuer 的部署查询登录方式
- **THEN** 报告 oidc=false、key=true
