# drivebridge-file-platform-consumers Specification

## Purpose
TBD - created by archiving change drivebridge-file-platform-consumers-v2. Update Purpose after archive.
## Requirements
### Requirement: 上传恢复交互
客户端 MUST 在创建请求前保存稳定身份并展示分阶段进度。

#### Scenario: 创建响应丢失且刷新页面
- **WHEN** 重新进入上传中心
- **THEN** 通过原键查询并重选同文件，保留原操作。

#### Scenario: 网络发送达到100%但仍验证
- **WHEN** 显示进度
- **THEN** 标记验证中而非上传完成。

### Requirement: 整理与交付确认
客户端 MUST 使用服务端计划revision和完整性状态，不以本地计算替代裁决。

#### Scenario: 确认前计划变化
- **WHEN** 点击执行
- **THEN** 显示stale并要求重新预览。

#### Scenario: 交付缺一项
- **WHEN** 点击发布
- **THEN** 显示缺项及恢复原任务入口，不发布不完整集合。

### Requirement: 检索和隐私体验
客户端 MUST 保留检索完整性与版本锚点，私人备注公开须显式预览。

#### Scenario: 语义返回20项
- **WHEN** 选择结果
- **THEN** 显示Top-K候选而非全量。

#### Scenario: 空间切换时旧查询返回
- **WHEN** 渲染结果
- **THEN** 丢弃旧响应，保留新空间上下文。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

