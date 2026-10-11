## Why

现有Finder、容量页和审计页已有基础；新增数据必须提供可追溯层次和状态，而不是堆叠更多图表或在客户端计算业务真值。

## What Changes

- 完善总览、文件详情与可钻取信息架构的加法合同、错误恢复、权限、状态和验收任务。
- 准入 `split-owner`；唯一实现owner为 `cli/drivebridge/web`，本文不把任务迁入根仓或复制到其他surface。
- 保留旧URI、工具名、操作身份与各领域local/S3/Capsa入口。所有新增能力为required、分阶段待实施。
- 本次只交付规格与文档；不实现产品代码、不修改真实配置、不接真实云账号或调用付费模型。

## Capabilities

### New Capabilities
- `drivebridge-information-workspace`：总览、文件详情与可钻取信息架构及本包正常、失败、权限、并发与恢复合同。

### Modified Capabilities
无既有规格的破坏性替换；旧行为兼容测试作为本包实施门。

## Impact

- owner `cli/drivebridge/web` 的应用合同及其文档；接口映射见 [设计](design.md)，原子任务见 [任务](tasks.md)。
- 跨owner范围与保留能力见 [根handoff](../../../../../openspec/changes/drivebridge-file-platform-evolution-v2/handoffs.md)。根包只维护设计关系，不记录本包实现完成状态。
- 规划HTTP/MCP/SDK、可选配置和数据模型均为加法；未启用时旧系统可运行，主程序保持pure Go（DuckDB为可选独立进程）。
