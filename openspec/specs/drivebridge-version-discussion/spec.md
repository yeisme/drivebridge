# drivebridge-version-discussion Specification

## Purpose
TBD - created by archiving change drivebridge-version-discussion-v1. Update Purpose after archive.
## Requirements
### Requirement: 版本线程与并发
系统 MUST 将线程绑定固定文件版本并对编辑执行revision检查。

#### Scenario: 文件产生新版本
- **WHEN** 读取旧线程
- **THEN** 保持旧锚点并提供明确旧版入口。

#### Scenario: 两个客户端同时编辑
- **WHEN** 第二个提交旧revision
- **THEN** revision_conflict且保留输入。

### Requirement: 私人备注与撤权
系统 MUST 同时验证作者与文件权限，公开必须显式确认。

#### Scenario: 作者查看自己的备注
- **WHEN** 仍有文件权限
- **THEN** 仅作者能读。

#### Scenario: 作者失去文件权限
- **WHEN** 读取备注或通知摘要
- **THEN** 拒绝正文，不从历史通知泄漏。

### Requirement: 讨论不授予领域批准
系统 MUST 将线程解决与内容owner审核独立。

#### Scenario: 用户解决线程
- **WHEN** 查询交付内容状态
- **THEN** 只改变线程，不改变领域review。

#### Scenario: 评论内包含脚本
- **WHEN** 展示正文
- **THEN** 作为纯文本，不执行。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

