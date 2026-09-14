## Why

CLI 传输命令（upload/download/ls/find/changes/trash/restore 等）只在本地模式可用；远程个人空间模式只有 `remote --action <tool>` + stdin JSON 的原始形态。配置文件里 `remote_url` 已是一等键，但传输命令不消费它——远程存储交互是"Agent 云存储工具"的明显缺口。

## What Changes（设计，实现归后续波）

- **路由规则**：CLI 传输/组织命令在解析后的 `remote_url` 非空时自动走远程 adapter（token/local-root 同源解析），本地模式零变化；输出合同同一（operation envelope）。
- **能力对齐**：远程工具面（capabilities/files/stat/upload/download/operations/resume/cancel）与本地命令一一映射；本地有而远程缺的（mkdir/search/changes/trash/restore）→ 远程走 backend HTTP API 对等端点（backend 侧另立 change 或复用既有 API 面），缺什么就明确 `remote_unsupported` 不静默降级。
- **同步终态语义**：远程 CLI 同样 awaitTerminal（轮询 operation_get），幂等键同一派生规则（space|file|name）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-remote-client`：新增 CLI parity 需求（本波仅设计）。

## Impact

不新增凭据面（token 文件同源）；远程模式无本地 state，find/changes 类索引功能在远程语义=服务端查询，不在 CLI 侧伪造本地索引。
