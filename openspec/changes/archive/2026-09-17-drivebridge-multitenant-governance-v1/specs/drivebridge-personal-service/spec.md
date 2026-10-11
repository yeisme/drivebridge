## MODIFIED Requirements

### Requirement: Deployment tenant boundary
未显式启用治理的部署 SHALL 保持实例即租户；通过部署侧治理引导后 SHALL 支持单实例多租户，并逐请求检查租户成员关系与资源归属。实例身份继续表示引用所属部署，不等同于已启用治理后的业务租户 ID。

#### Scenario: Cross-instance reference
- **WHEN** 请求携带的引用实例身份与当前部署不匹配
- **THEN** 返回拒绝且不泄露目标内容或存在性

#### Scenario: Tenant profile is read-only metadata
- **WHEN** 旧部署配置租户名称等 profile 后在线查询
- **THEN** 既有 profile 仍为只读部署元数据，不获得新的治理角色

#### Scenario: Multi-tenant scaling path
- **WHEN** 需要服务多个租户且治理未启用
- **THEN** 按实例分片部署（独立 state/origin/issuer），不承诺单实例内租户化或跨实例数据迁移

#### Scenario: Explicit multi-tenant activation
- **WHEN** 部署操作者指定首批最高管理员并启用治理
- **THEN** 旧数据进入默认租户，其他租户通过独立成员关系隔离，原引用与文件 ID 保留

### Requirement: Admin session inheritance
未启用治理的部署中，admin 用户密钥会话 SHALL 保留旧权限投影；启用治理后 SHALL 以实时平台及租户角色为权限来源，不把旧会话 admin 标记自动升级。

#### Scenario: Admin session reaches management surface
- **WHEN** admin 密钥换发的会话调用用户管理接口
- **THEN** 与 admin 密钥 Bearer 同权限

#### Scenario: Plain session rejected
- **WHEN** 非 admin 会话或非 admin 密钥调用用户管理接口
- **THEN** 返回 forbidden

#### Scenario: Legacy deployment
- **WHEN** 未启用治理的 admin 会话调用管理面
- **THEN** 与旧 admin 密钥 Bearer 同权限

#### Scenario: Governance activation
- **WHEN** 治理已启用，旧 admin 会话没有显式角色
- **THEN** 不获得管理读取或应急权限

### Requirement: User management HTTP surface
系统 SHALL 保留旧列表响应形状且不返回凭据或哈希；治理启用后列表仅平台最高管理员可读，旧签发、轮换及吊销管理写接口 SHALL 拒绝并引导到账号激活、恢复与冻结流程。

#### Scenario: Admin lists users
- **WHEN** admin 调用用户列表接口
- **THEN** 返回全部用户密钥的生命周期元数据（id/用户/名称/admin/过期/吊销），不含哈希或明文密钥

#### Scenario: Admin revokes a key
- **WHEN** admin 调用吊销接口指定密钥 id
- **THEN** 该密钥下一次认证被拒，接口返回成功

#### Scenario: Legacy admin revokes a key
- **WHEN** 未启用治理的 admin 调用旧吊销接口
- **THEN** 该密钥下一次认证被拒，原合同保持不变

#### Scenario: Governance recovery boundary
- **WHEN** 已启用治理的管理员试图通过旧管理写接口取得他人新密钥
- **THEN** 返回 account_recovery_required，不执行写入

### Requirement: Key rotation
部署侧密钥原子换发 SHALL 保留同一用户身份及旧密钥失效语义；治理启用后的在线恢复 SHALL 由本人领取新凭据，管理员不能通过旧在线轮换接口取得密钥。

#### Scenario: Rotate keeps identity
- **WHEN** 管理员轮换某用户的密钥
- **THEN** 新密钥立即以同一用户身份工作，旧密钥立即失效，用户与其数据归属不变

#### Scenario: Operator recovery
- **WHEN** 部署操作者在本机执行明确的密钥恢复
- **THEN** 用户与数据归属不变，不将该部署侧信任边界伪装成普通 Web 权限

#### Scenario: Owner claims recovered credential
- **WHEN** 本人提交有效领取证明且恢复已获批
- **THEN** 旧密钥、会话及未完成授权失效，人工恢复的特权需独立确认
