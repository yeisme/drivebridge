## ADDED Requirements

### Requirement: 冻结同步计划
系统 MUST 执行前重新验证两端基线与根身份，变化后拒绝旧计划。

#### Scenario: 完整扫描后无变化
- **WHEN** 用户终端apply
- **THEN** 按冻结集合执行并保留原操作。

#### Scenario: 确认后远端新增或修改
- **WHEN** apply旧计划
- **THEN** plan_stale，原件不被覆盖。

### Requirement: 单写入者与冲突
系统 MUST 保留冲突双份且不得以失联证明旧设备已停止。

#### Scenario: 同一文件双端修改
- **WHEN** 执行同步
- **THEN** 保留两份并显示来源。

#### Scenario: 另一设备离线
- **WHEN** 请求接管
- **THEN** 要求明确交接，不自动取得写入权。

### Requirement: 分类清理与耐久
系统 MUST 区分未刷写缓存和可重建缓存，挂载存活不得证明远端保存。

#### Scenario: VFS仍有待提交字节
- **WHEN** 预览清理
- **THEN** 受保护，不自动删。

#### Scenario: 扫描只得到部分条目
- **WHEN** 计算远端删除
- **THEN** 拒绝传播删除并显示coverage。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
