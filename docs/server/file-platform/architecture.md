# 文件平台数据层与运行时（设计）

## 状态与边界

2026-09-25 原设计记录保留；2026-10-01 定向状态纠偏：当前源码已存在 PostgreSQL adapter、团队运行时和 metadata 迁移入口，不能笼统称未实现。历史本地验证见 [runtime-status](runtime-status.md)，本次没有重跑业务测试，不将源码/历史记录升级为当前部署、多实例或规模通过。ClickHouse、DuckDB 与新增 API 按各 owner change 独立记录状态。个人工作台采用 [SQLite 单机优先选型](../../architecture/personal-workspace-technical-decisions.md)，保留 PostgreSQL 兼容，不要求每个新切片双数据库同步验收。

| 层 | canonical状态 | 可重建/依赖 | 禁止承担 |
| --- | --- | --- | --- |
| SQLite单机 | 当前文件、版本、权限、操作 | 既有单进程服务 | 多机writer承诺 |
| PostgreSQL团队 | 同一业务模型、Job/Outbox/Lease/Policy | 同owner API+worker，GORM | 新独立产品、通用队列平台 |
| ClickHouse | 无业务真源；文件最新投影/传输审计事实/容量趋势 | generation+watermark+event去重 | 配额批准、权限、删除资格 |
| DuckDB | 显式授权的版本化快照 | 可选受控进程，无网络和自由SQL | 在线事务、多租户权限数据库 |
| Manju | 资产理解、证据、语义索引生命周期 | 对接既有WeKnora/Qdrant/producer | DriveBridge文件迁移和网盘凭据 |

## 默认与接口

默认保留SQLite及旧上传限制；团队PostgreSQL和分析连接是加法配置，经owner CLI/API创建，凭据按现有私有配置规则存放。主程序保持CGO_ENABLED=0。DuckDB Go驱动需要cgo，因此可选进程与主程序分离；依赖缺失明确不可用，不自动安装。依据：[DuckDB Go客户端](https://duckdb.org/docs/stable/clients/go)。

ClickHouse元数据文本索引以官方GA 26.2为能力基线，实际部署探测及中文测试另验；不能把其文本索引称为多模态语义引擎。依据：[官方文本索引](https://clickhouse.com/docs/engines/table-engines/mergetree-family/textindexes)。

## 提交与投影

GORM事务提交文件/操作和Outbox。worker以lease+fencing领取；外部副作用不因lease过期而自动重做。分析消费者幂等应用event/resource revision，墓碑优先，checkpoint与generation可核对。新代次先backfill再追增量，校验后切换，不混查询旧新代次。

权限由事务服务即时裁决；候选检索前过滤，返回前复查。聚合/facet不能靠结果后过滤补救；授权投影过期就安全回退或拒绝。跨owner映射不按邮箱推断，caller只是来源标签。

## 实施入口

- [团队运行时](../../../openspec/changes/drivebridge-team-runtime-projections-v1/design.md)
- [分析查询](../../../openspec/changes/drivebridge-analytics-search-v1/design.md)
- [内容绑定](../../../openspec/changes/drivebridge-content-retrieval-v1/design.md)
- [迁移与容量验收](verification.md)
