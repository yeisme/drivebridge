## Why

本机 `drivebridge bind` 已冻结 purpose 白名单、前缀围栏与撤销停读。Server 现有 `handoff-binding` 仍只把 `reference` 当常规用途，且内容读取路径对「文件已移出绑定前缀」的复核不如 `GetHandoff` 明确。需要在不替换既有 handoff API 的前提下与本机 bind 对齐。

## What Changes

- purpose 加法白名单：在保留 `reference` 的同时接受 `vault`、`notes-attachments`、`materials`、`saveback`、`owner-store`；未知 purpose 稳定拒绝（`invalid_binding`）。
- `OpenHandoff` / `readableHandoff` 按文件**当前** path 与 space 复核绑定前缀；越界返回既有 `forbidden`，不重命名错误码。
- disable 后 `BindReference` 与 `OpenHandoff` 均失败；不声称删除已导入副本或源文件。
- 调用方 `consumer` 标签不是认证：Agent 不能 `CreateHandoffBinding`；授权仍是 BindingID + 已登记 AgentID。
- 可选加法：同一 owner+agent+space+prefix+consumer+project+purpose 的未 disable 绑定复用；不要求客户端新增幂等键。
- CLI `--purpose` help 列出允许值。不替换 create/list/disable、BindReference、GetHandoff、OpenHandoff。

## Capabilities

### New Capabilities
- `handoff-binding-prefix`：Server 接收绑定的 purpose 白名单、读取时前缀复核、disable 停新读、标签非认证。

### Modified Capabilities
- `creative-handoff`：保留既有 `reference` 与 create/list/disable；内容读取与 GetHandoff 同样按当前路径拒绝越界。

## Impact

Owner：`backend-server/drivebridge`。触及 `internal/service/handoffs.go` 与测试、SDK 加法常量、CLI help、`docs/creative-resource-handoff.md` 增量。不改 MCP 本机 `bind` CLI、网页、grant ID 凭证语义、MCP 签名 URL。不接真实云账号。
