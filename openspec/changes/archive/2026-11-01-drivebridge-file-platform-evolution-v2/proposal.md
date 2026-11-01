## Why

DriveBridge 已有文件、版本、传输、分享、团队与审计能力，但跨设备恢复、云端直管、百万文件查询、Agent整理以及内容检索缺少一致的阶段合同。用户已确认同时完善规格、文档和各owner实施任务，不能以单一上传MVP替代完整能力账本。

## What Changes

- 明确DriveBridge文件平面、Client体验、Manju内容理解、Anatomia视频与Sonora音频的split-owner边界；已有三仓保持，不新建第四个实现副本。
- 建立14项必需能力及4个配套owner包，定义恢复、上传、三云管理、虚拟组织、分析、内容检索、Agent计划、可视化、交付、讨论、同步与自动化。
- SQLite保留单机；PostgreSQL团队事务；ClickHouse在线分析；DuckDB受控快照；Manju语义检索。分析投影不成为事务/授权真源。
- 百万文件、千万事件、万文件批次和100GiB单文件均是待验收目标；不变更当前默认上限。
- 本轮只生成规格与文档，所有实现任务未勾选。产品代码、配置、数据迁移、真实账号、模型、发布与部署均未执行。

## Capabilities

### New Capabilities
- `drivebridge-file-platform-owner-contract`：必需能力保留、owner分工、跨项目版本授权交接、数据层与证据门。

### Modified Capabilities
不覆盖既有owner规格；旧URI、操作ID、工具、存储入口与权限语义保持，新增接口按owner加法设计。

## Impact

- `cli/drivebridge/server`：13个服务端包（含独立保留保护）；`cli/drivebridge`：同步与远程消费2包；`cli/drivebridge/web`：信息架构与流程消费2包；`backend-server/manju`：新联调1包。
- [handoffs.md](handoffs.md) 是路由；实现进度只在各包tasks.md。本根tasks只记录设计、文档与治理验证。
- 本包不迁移canonical状态、不合并子仓、不增加新云盘种类，也不重建Manju的索引/主体/知识模型。
