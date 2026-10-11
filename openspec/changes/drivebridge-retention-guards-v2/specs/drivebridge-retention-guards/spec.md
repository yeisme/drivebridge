## ADDED Requirements

### Requirement: 保护与清理互斥
系统 MUST 在同一事务裁决版本保护与purge资格。

#### Scenario: 活跃交付hold
- **WHEN** 执行清理
- **THEN** 跳过并说明保护来源。

#### Scenario: purge与hold并发
- **WHEN** 双方提交
- **THEN** 只有一方获得资格，不产生悬空保证。

### Requirement: 容量和引用安全
系统 MUST 检查所有有效引用后才删除共享blob，并明确保留策略的容量影响。

#### Scenario: 两个版本引用同blob
- **WHEN** 清理其中一个版本
- **THEN** 另一版本可读，物理字节保留。

#### Scenario: 分析投影显示零引用但事务仍有引用
- **WHEN** 执行清理
- **THEN** 以事务真源拒绝删除。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
