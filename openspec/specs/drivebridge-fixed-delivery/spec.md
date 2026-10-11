# drivebridge-fixed-delivery Specification

## Purpose
TBD - created by archiving change drivebridge-fixed-delivery-v2. Update Purpose after archive.
## Requirements
### Requirement: 固定清单完整性
系统 MUST 仅在全部必需成员与清单核验完成后报告完整交付。

#### Scenario: 十项全部校验且保留保护成功
- **WHEN** 发布清单
- **THEN** 完整且版本固定。

#### Scenario: 十项缺一项
- **WHEN** 发布清单
- **THEN** delivery_incomplete，不能自动删掉缺项。

### Requirement: 接收事实分级
系统 MUST 区分页面访问、字节传出、接收端校验和明确确认。

#### Scenario: 浏览器触发下载
- **WHEN** 记录事件
- **THEN** 最多bytes_served，不自动receiver_confirmed。

#### Scenario: 未认证接收者提交验收
- **WHEN** 创建确认回执
- **THEN** 拒绝冒充身份，已传出事实保留。

### Requirement: 撤销和领域采纳
系统 MUST 撤销未来受控访问且不改变原领域审核结果。

#### Scenario: 分享撤销
- **WHEN** 再次请求字节
- **THEN** 拒绝读取。

#### Scenario: 接收方已下载
- **WHEN** 用户撤销分享
- **THEN** 说明已取得副本不可收回，不能标领域审核通过。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

