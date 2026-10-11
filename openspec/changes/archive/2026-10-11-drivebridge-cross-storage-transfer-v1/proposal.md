## Why

目录作业、archive与relay已有各自范围；跨云迁移需要稳定计划和对来源清理的独立授权，不能把复制成功误当迁移完成。

## What Changes

- 完善跨存储复制、迁移与逐项恢复的加法合同、错误恢复、权限、状态和验收任务。
- 准入 `fit`；唯一实现owner为 `cli/drivebridge/server`，本文不把任务迁入根仓或复制到其他surface。
- 保留旧URI、工具名、操作身份与各领域local/S3/Capsa入口。所有新增能力为required、分阶段待实施。
- 本次只交付规格与文档；不实现产品代码、不修改真实配置、不接真实云账号或调用付费模型。

## Capabilities

### New Capabilities
- `drivebridge-cross-storage-transfer`：跨存储复制、迁移与逐项恢复及本包正常、失败、权限、并发与恢复合同。

### Modified Capabilities
无既有规格的破坏性替换；旧行为兼容测试作为本包实施门。

## Impact

- owner `cli/drivebridge/server` 的应用合同及其文档；接口映射见 [设计](design.md)，原子任务见 [任务](tasks.md)。
- 跨owner范围与保留能力见 [根handoff](../../../../../openspec/changes/drivebridge-file-platform-evolution-v2/handoffs.md)。根包只维护设计关系，不记录本包实现完成状态。
- 规划HTTP/MCP/SDK、可选配置和数据模型均为加法；未启用时旧系统可运行，主程序保持pure Go（DuckDB为可选独立进程）。
