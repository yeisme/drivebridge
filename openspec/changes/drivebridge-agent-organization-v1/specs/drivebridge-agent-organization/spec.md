## ADDED Requirements

### Requirement: 风险分级与不可自批
系统 MUST 按动作、范围、额度和有效期校验预授权；Agent不得确认自己的高风险计划。

#### Scenario: 规则允许目录内标签整理
- **WHEN** Agent执行固定计划
- **THEN** 完成授权标签动作并留逐项回执。

#### Scenario: 计划包含删除或跨空间外发
- **WHEN** Agent尝试直接执行
- **THEN** approval_required，已有write不替代确认。

### Requirement: 计划冻结与再校验
系统 MUST 将批准绑定确切revision、资源版本及规则，不自动扩大选择集合。

#### Scenario: 用户确认十个文件
- **WHEN** 执行已冻结计划
- **THEN** 只处理这十个固定项。

#### Scenario: 确认后文件或policy变化
- **WHEN** 执行原计划
- **THEN** plan_stale，不静默重新筛选。

### Requirement: 部分成功和补偿
系统 MUST 保留逐项结果，补偿应为独立可审计划并检查后续变化。

#### Scenario: 部分移动成功
- **WHEN** 恢复原计划
- **THEN** 成功项跳过，失败项重新授权后恢复。

#### Scenario: 文件后来又被修改
- **WHEN** 执行补偿
- **THEN** compensation_conflict，后续内容保持。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
