## ADDED Requirements

### Requirement: 复制与来源清理分离
系统 MUST 在目标核验完成后才允许独立授权的来源清理。

#### Scenario: 迁移复制成功
- **WHEN** 用户确认清理来源
- **THEN** 只软删除仍匹配原版本的来源。

#### Scenario: 来源删除失败
- **WHEN** 读取迁移结果
- **THEN** copied_source_pending，目标副本不重传。

### Requirement: 逐项幂等与结果未知
系统 MUST 保留固定逐项映射和原操作身份，未知副作用先对账。

#### Scenario: 目标完成响应丢失
- **WHEN** 恢复同一项
- **THEN** 确认目标摘要后复用原文件。

#### Scenario: 来源在恢复前更新
- **WHEN** 继续原计划
- **THEN** source_changed，暂停该项并保留其他已完成结果。

### Requirement: 跨空间权限与补偿
系统 MUST 单独校验源读取、跨空间传出、目标写入及来源删除，不从copy权限推导全部权限。

#### Scenario: 只有读取和目标写入
- **WHEN** 执行copy
- **THEN** 允许授权复制，不提供来源清理。

#### Scenario: 目标后来被编辑
- **WHEN** 请求补偿
- **THEN** 生成冲突待确认，不覆盖后续版本。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
