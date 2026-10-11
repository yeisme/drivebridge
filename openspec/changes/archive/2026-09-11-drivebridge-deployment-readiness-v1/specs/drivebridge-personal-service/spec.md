## ADDED Requirements

### Requirement: Deployment preflight
doctor SHALL 以只读方式报告部署就绪项，按名区分 pass/fail/info；不得获取实例锁、修改状态或输出凭据；state 不一致时给出恢复指引而不是重新生成的暗示。

#### Scenario: Consistent existing state
- **WHEN** state 三件（instance.key、instance.id、metadata.sqlite）一致且平台存储已配置
- **THEN** 相关检查项汇总 pass，命令以 0 退出

#### Scenario: Key or instance identity missing while database exists
- **WHEN** metadata.sqlite 存在但 instance.key 或 instance.id 缺失/形状不符
- **THEN** 对应检查项按名失败并指向秘密备份恢复路径，不以非 0 之外的任何方式掩盖或触发重新生成

#### Scenario: Optional issuer probe failure does not block
- **WHEN** 传入 `--issuer` 且 discovery 端点不可达或 issuer 字段不匹配
- **THEN** 仅 oidc_issuer 项失败，其余检查照常报告，命令整体 not ready
