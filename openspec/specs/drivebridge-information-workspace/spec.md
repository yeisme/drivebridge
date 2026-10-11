# drivebridge-information-workspace Specification

## Purpose
TBD - created by archiving change drivebridge-information-workspace-v2. Update Purpose after archive.
## Requirements
### Requirement: 总览与钻取同口径
客户端 MUST 对每张图标明范围、观测时间和缺失状态，钻取使用同一服务查询条件。

#### Scenario: 选择传输失败柱
- **WHEN** 进入明细
- **THEN** 筛选不变并能打开原Operation。

#### Scenario: 后端只有部分扫描
- **WHEN** 绘制容量
- **THEN** 显示partial与观测范围，不称全盘总量。

### Requirement: 文件详情与版本
客户端 MUST 清晰区分版本、来源、理解、讨论和原操作，不把最新版替代所选版本。

#### Scenario: 从旧交付打开文件
- **WHEN** 显示详情
- **THEN** 版本选择钉住原ref。

#### Scenario: 该版本不可读
- **WHEN** 打开内容
- **THEN** 显示版本不可用与恢复入口，不偷换新文件。

### Requirement: 权限和响应交互
客户端 MUST 丢弃过期上下文响应并覆盖键盘、移动端和不可用状态。

#### Scenario: 查询中切换租户
- **WHEN** 旧请求返回
- **THEN** 不渲染旧租户数据。

#### Scenario: 仅键盘或390px屏幕
- **WHEN** 浏览图表和详情
- **THEN** 可进入数据表、关闭详情并回原焦点，无必要控件裁切。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

