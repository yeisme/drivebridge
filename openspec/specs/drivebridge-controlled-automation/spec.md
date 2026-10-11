# drivebridge-controlled-automation Specification

## Purpose
TBD - created by archiving change drivebridge-controlled-automation-v1. Update Purpose after archive.
## Requirements
### Requirement: 持久投递与幂等
系统 MUST 对已提交事件至少一次投递，并按event/rule revision保存唯一意图。

#### Scenario: 重复投递同事件
- **WHEN** 规则消费
- **THEN** 复用原意图与原Operation。

#### Scenario: 提交后进程崩溃
- **WHEN** 重启消费
- **THEN** 事件可恢复，不丢业务变化。

### Requirement: 游标与撤权
系统 MUST 处理游标过期及投递后撤权，不依赖历史事件授予访问。

#### Scenario: 游标过期
- **WHEN** 请求下一页
- **THEN** cursor_expired并提供受控重建入口。

#### Scenario: 投递后Agent撤权
- **WHEN** 消费文件引用
- **THEN** 拒绝读取与执行，事件不泄漏正文。

### Requirement: 自动化边界与暂停
系统 MUST 不从文件事件推导付费、删除或外部通信许可。

#### Scenario: 规则要求付费理解但无额度授权
- **WHEN** 执行意图
- **THEN** 等待对应owner准入。

#### Scenario: 相同确定失败连续三次
- **WHEN** 调度规则
- **THEN** 暂停并保留原记录，不无限创建任务。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

