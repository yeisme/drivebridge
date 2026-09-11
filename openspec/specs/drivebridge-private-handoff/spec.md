# drivebridge-private-handoff Specification

## Purpose
TBD - created by archiving change drivebridge-host-handoff-v1. Update Purpose after archive.
## Requirements
### Requirement: Stable identity and private transport
系统 SHALL 使用稳定文件引用并隔离空间、身份、目录和文件版本，临时下载 URL 只交给已配置的本机 owner adapter。

#### Scenario: Allowed reference
- **WHEN** 原任务身份有权消费指定空间的固定版本文件
- **THEN** 系统返回稳定引用与校验摘要，在实际提交前通过私有通道获取下载授权

#### Scenario: Wrong identity or replaced file
- **WHEN** 访问者身份、空间或文件版本不匹配
- **THEN** 系统拒绝交接，不返回下载 URL

### Requirement: Recovery does not repeat generation
系统 SHALL 使用原操作/原任务进行恢复，禁止因网络错误或回存失败自动发起新的生成。

#### Scenario: Lost upload acknowledgement
- **WHEN** 上传已完成但客户端没有取得回执
- **THEN** 恢复核对原操作的远端文件摘要并返回同一文件，不重复创建资产

#### Scenario: Output save fails
- **WHEN** 成功生成任务的成品回存失败
- **THEN** 只恢复文件传输，保留原领域任务与审核状态

### Requirement: Explicit activation and honest readiness
系统 SHALL 默认关闭新增写入口，并区分本地 fixture 验证与真实账号、Gateway、Provider 区域验收。

#### Scenario: Secure handoff absent
- **WHEN** 宿主没有可用的私有交接通道
- **THEN** 系统明确拒绝参考交接，不把 URL 写入普通 MCP 输出，也不自动配置 TOS

