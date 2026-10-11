## Why

Agent 上传与导入的产物只散落在传输记录里，用户无法从"结果"视角挑选一个固定版本收进自己的空间；既有 `link` 是零拷贝引用，满足不了"长期持有受管副本"的需要。

## What Changes

- 新增收件箱查询（HTTP `GET /api/v1/inbox`、MCP 工具 `inbox`）：列出调用者近 30 天的操作，支持 status/caller/space 过滤，结果按当前身份读权限后置过滤。
- 新增受管归档（HTTP `POST /api/v1/archive`、MCP 工具 `archive`、SDK `Inbox`/`Archive`）：把指定固定版本经服务端复制进目标空间；幂等键去重、路径冲突自动改名绝不覆盖既有文件、期望头版本冲突拒绝、中断可 resume。
- `Operation` 仅新增 `source_id`/`source_version`（omitempty）字段，不改既有字段。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-service`：新增收件箱与受管归档需求。

### New Capabilities

无。

## Impact

backend-server/drivebridge 本地全量实现与验收；generic 桥 inbox 工具归 mcp owner change；产物收件箱页与归档表单归 client owner change。无契约删除或重命名，非断代演进。
