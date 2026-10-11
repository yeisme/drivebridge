## ADDED Requirements

### Requirement: 批次与原幂等身份
系统 MUST 在响应丢失后使用同键同指纹找回或重放原任务；批次只引用原子 Operation。

#### Scenario: 创建响应丢失
- **WHEN** 客户端调用 lookup 并恢复
- **THEN** 取得唯一操作；不会新增版本或重复预留。

#### Scenario: 同键不同源摘要
- **WHEN** 重放创建
- **THEN** idempotency_conflict，保留原任务。

### Requirement: 耐久分片和源校验
系统 MUST 只确认已耐久存储并有摘要证据的分片，重新选择来源时核验原指纹。

#### Scenario: 100GiB传输中断
- **WHEN** 重启并读回 received 范围
- **THEN** 恢复缺失范围且最终读回摘要一致。

#### Scenario: 选择同名不同内容
- **WHEN** 恢复上传
- **THEN** content_mismatch，既有分片不被替换。

### Requirement: 批次部分失败与取消
系统 MUST 分页返回逐项终态并独立释放取消项预留，不把部分成功显示为完整。

#### Scenario: 万项中十项失败
- **WHEN** 读取批次并恢复
- **THEN** 已成功项不重写，只处理失败/未执行且仍授权的项。

#### Scenario: 提交与取消竞态
- **WHEN** 取消遇到已提交文件
- **THEN** 保留文件并报告已完成副作用，不声称撤回。

### Requirement: 协议兼容与后端重组
系统 MUST 保留旧8MiB协议并按后端约束重组，不能共用未经验证的云分片编号。

#### Scenario: 旧单文件客户端
- **WHEN** 上传小文件并 complete
- **THEN** 原返回字段与状态仍兼容。

#### Scenario: 100GiB使用S3
- **WHEN** 规划后端分片
- **THEN** 分片数在后端能力限制内，单文件不整块入内存。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
