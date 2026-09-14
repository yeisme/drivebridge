## Why

generic 桥的 `operations` 面向恢复语义（原操作续传），Agent 与用户缺一个"最近发生了什么"的只读浏览入口：产物何时完成、哪些传输失败，需要翻操作语义查询。

## What Changes

新增只读工具 `inbox`：列出本机状态库近 30 天内更新过的完成或失败操作，按更新时间倒序，上限 200 条；状态库不可用时返回 state_unavailable，不以空列表冒充。

## Capabilities

### Modified Capabilities

- `drivebridge-local-files`：新增近窗操作收件箱需求。

### New Capabilities

无。

## Impact

mcp/drivebridge generic 桥本地实现与验收；不改动既有工具与引用合同，服务端收件箱/归档归 backend owner change `drivebridge-agent-artifact-archive-v1`。
