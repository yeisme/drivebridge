## ADDED Requirements

### Requirement: Deployment tenant boundary
部署实例 SHALL 即租户：实例身份是租户唯一标识，跨实例访问仅经目标实例签发的限定 Agent 凭据，不提供实例间委托、联邦或共享。

#### Scenario: Cross-instance reference
- **WHEN** 请求携带的引用实例身份与当前部署不匹配
- **THEN** 返回拒绝且不泄露目标内容或存在性

#### Scenario: Tenant profile is read-only metadata
- **WHEN** 部署配置租户名称等 profile 后在线查询
- **THEN** 登录用户可读、任何在线面不可改；修改仅经停机改启动参数

#### Scenario: Multi-tenant scaling path
- **WHEN** 需要服务多个租户
- **THEN** 按实例分片部署（独立 state/origin/issuer），不承诺单实例内租户化或跨实例数据迁移
