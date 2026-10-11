## ADDED Requirements

### Requirement: 虚拟组织与标签权威
系统 MUST 将文件标签和集合持久化于Server，并区分人工、规则与建议来源。

#### Scenario: Agent建议分类
- **WHEN** 写suggested标签
- **THEN** 文件路径不变且人工标签不被覆盖。

#### Scenario: 模型重跑
- **WHEN** 生成不同分类
- **THEN** 保留人工决定与拒绝记录。

### Requirement: 集合与快照语义
系统 MUST 区分动态成员和固定版本快照，并在每次读取时重新授权。

#### Scenario: 文件更新
- **WHEN** 读取动态集合与旧快照
- **THEN** 动态集合显示新版，旧快照仍指原版本。

#### Scenario: 成员失权
- **WHEN** 读取集合计数与成员
- **THEN** 均过滤失权资源，旧快照不扩权。

### Requirement: 精确重复和相关候选
系统 MUST 仅将已验证同摘要标记为精确重复，不将相似结果当可删除副本。

#### Scenario: 同摘要两个可读文件
- **WHEN** 查询重复候选
- **THEN** 显示两个独立引用和来源。

#### Scenario: 只有相近名称或描述
- **WHEN** 查询候选
- **THEN** 标记similar或unknown，不能自动purge。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
