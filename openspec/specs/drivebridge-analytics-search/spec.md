# drivebridge-analytics-search Specification

## Purpose
TBD - created by archiving change drivebridge-analytics-search-v1. Update Purpose after archive.
## Requirements
### Requirement: 事务与分析独立
系统 MUST 在ClickHouse不可用时保持文件事务可用，并明示查询降级而非编造零值。

#### Scenario: ClickHouse故障
- **WHEN** 读取资源页
- **THEN** 显示分析不可用，可安全读取当前文件。

#### Scenario: 数据未采集
- **WHEN** 绘制趋势
- **THEN** 保留缺口，不补0。

### Requirement: 安全查询和分面
系统 MUST 对查询、计数、分面及导出施加当前授权，失效授权投影不得继续返回聚合。

#### Scenario: 目录受限Agent
- **WHEN** 查询大小分布
- **THEN** 只统计该目录，结果可钻取同范围。

#### Scenario: 撤权事件尚未投影
- **WHEN** 请求旧facet缓存
- **THEN** 拒绝或安全回退，不泄露计数。

### Requirement: 幂等投影与代次
系统 MUST 在重复/乱序/重建后对同一冻结数据集产生一致结果。

#### Scenario: 重放相同事件
- **WHEN** 重建新generation
- **THEN** 计数和字节不增加。

#### Scenario: 旧修改晚于墓碑到达
- **WHEN** 更新file_current
- **THEN** 不复活已删资源。

### Requirement: DuckDB进程隔离
系统 MUST 保持默认CGO_ENABLED=0构建，DuckDB只处理受控授权快照。

#### Scenario: 可选DuckDB缺失
- **WHEN** 执行快照查询
- **THEN** analytics_backend_unavailable，不影响在线上传。

#### Scenario: 查询包含SQL或任意路径
- **WHEN** 提交查询
- **THEN** invalid_query，进程不访问请求指定文件。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

