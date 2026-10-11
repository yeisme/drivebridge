# drivebridge-remote-surfaces Specification

## Purpose
TBD - created by archiving change drivebridge-remote-surfaces-v2. Update Purpose after archive.
## Requirements
### Requirement: 共享合同与原ID
适配器 MUST 消费Server公共SDK并保留其资源和操作身份，不复制远程规则。

#### Scenario: 远程上传响应丢失
- **WHEN** 同键lookup
- **THEN** 恢复原Server Operation。

#### Scenario: Server新工具缺失
- **WHEN** 用户请求该动作
- **THEN** unsupported并给出实际缺口，不猜别名。

### Requirement: 本机文件与纯MCP
适配器 MUST 区分客户端文件访问与远程控制面。

#### Scenario: 远程Agent没有本机adapter
- **WHEN** 请求上传本机路径
- **THEN** 说明网页/HTTP入口，不假装可读。

#### Scenario: 本机来源越过root
- **WHEN** 执行读取
- **THEN** 拒绝且不传输字节。

### Requirement: 可选快照分析
适配器 MUST 使用同一快照合同且不默认安装依赖或扩大导出范围。

#### Scenario: 已授权快照且DuckDB可用
- **WHEN** typed查询
- **THEN** 返回同代次统计并声明离线副本。

#### Scenario: 没有export授权
- **WHEN** 请求下载快照
- **THEN** 拒绝，不退化为全库导出。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

