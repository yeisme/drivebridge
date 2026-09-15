## Why

`find` 是纯索引（数据库投影）查询，却在排他进程锁被 MCP 客户端持有时与写命令一起死于 `state_locked`——与 recent/operation 已有的快照纪律不一致；Agent 在 MCP 运行中连按名检索都做不了。

## What Changes

`find` 在 `state_locked` 时经 GORM 只读快照（`mode=ro`，无迁移无写路径）执行与 `Service.Search` 相同语义的过滤/排序查询；投影标注 `data.read_only=true`，人读摘要明示快照来源与「命中仍需 stat 验证」。`ls`/`stat`/`changes` 保持排他——它们的价值就是活的远端验证。

## Capabilities

### Modified Capabilities

- `drivebridge-local-files`：Workspace directories and transfer CLI 需求增加索引检索持锁快照场景。

## Impact

mcp/drivebridge；零契约变更（信封增量字段 read_only）。
