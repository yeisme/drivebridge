## ADDED Requirements

### Requirement: 事务与投影分层
系统 MUST 将文件、权限、配额和操作留在事务真源，分析故障不得回滚已成功的文件业务事务。

#### Scenario: 分析库失联
- **WHEN** 上传完成并写 Outbox
- **THEN** 文件可读，投影显示滞后，事件可恢复。

#### Scenario: 数据库业务提交失败
- **WHEN** worker 准备投影
- **THEN** 不得发布未提交业务事件。

### Requirement: 原任务与 fencing
系统 MUST 防止失效租约的 worker 发布元数据，恢复先对账原外部操作。

#### Scenario: worker 崩溃后接管
- **WHEN** 新 worker 领取原 job
- **THEN** 继续原 Operation 且不重复发布版本。

#### Scenario: 旧 worker 晚到
- **WHEN** 持旧 token 写结果
- **THEN** 拒绝写入并保留诊断，不覆盖新状态。

### Requirement: 迁移与代次切换
系统 MUST 在可验证水位完成迁移及投影切换，不修改稳定资源身份。

#### Scenario: 新代次追平
- **WHEN** 比较同一水位数据
- **THEN** 数量、摘要及墓碑一致后切换。

#### Scenario: 切换后已有新写入
- **WHEN** operator 请求回旧库
- **THEN** 拒绝直接回旧快照，要求反向迁移或向前修复。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
