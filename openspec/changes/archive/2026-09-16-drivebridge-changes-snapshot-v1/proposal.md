## Why

`changes` 与 find 一样是纯数据库投影（events 表+游标），却在 MCP 客户端持锁时死于 state_locked；事件流是 Agent 对账的基础入口，持锁不可用破坏轮询节奏。

## What Changes

`changes` 在 state_locked 时经 GORM 只读快照执行与 `Service.Changes` 相同的游标语义查询；投影标注 `data.read_only=true`，摘要明示快照来源。cursor_ahead 等错误语义不变。

## Capabilities

### Modified Capabilities

- `drivebridge-local-files`：Workspace directories and transfer CLI 需求增加事件流持锁快照场景。

## Impact

mcp/drivebridge；零契约变更（增量 read_only 字段）。
