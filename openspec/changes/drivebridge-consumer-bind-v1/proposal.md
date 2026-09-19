## Why

根包 `drivebridge-consumer-binding-v1` 要求本机把项目绑到 DriveBridge 空间、允许前缀与 purpose。当前只有 kind 目录的 `bind_default_kinds` 与 `storage adopt`，没有绑定记录、默认发现过滤、前缀围栏或撤销。没有 bind，消费者无法把 list/stat/mutate 限制在已批准前缀内。

## What Changes

- 新增 `drivebridge bind discover|add|list|show|revoke`：CLI/application service 写入绑定记录（consumer、project_ref、space、prefix、purpose、可选 connection_id、幂等键）。
- 默认 `bind discover` 只列出 `bind_default` kinds（local/s3/onedrive/gdrive）以及已 adopt 的空间；逃生舱需 `--allow-generic`。绑定逃生舱空间同样默认拒绝。
- 带 consumer+project 或 bind-id 的 list/stat/upload/delete/mkdir 对越界前缀返回 `bind_prefix_denied`。未带绑定范围的既有 relay/upload/space 命令保持原义。
- 撤销后该 bind 的新 list/stat/import 拒绝；不声称收回 owner 文件或删除空间/adopt 记录。
- 调用方自称的 consumer/project 是存储事实，不是认证。
- MCP capabilities 加法 `bind_discover` / `bind_list` 只读工具。不打印凭据、密钥文件或签名 URL。

## Capabilities

### New Capabilities
- `drivebridge-consumer-bind`：本机 consumer bind 记录、默认发现、前缀围栏、撤销与只读 MCP 发现。

### Modified Capabilities
- （无）未绑定的 `drivebridge-local-files` 行为保持原义；围栏只作用于带 bind 范围的操作。

## Impact

Owner：`mcp/drivebridge`。触及 bind CLI、bridge 记录与前缀围栏、MCP 加法工具、recovery 错误码与 recipe 文档。不改 Server handoff-binding、`storage adopt` 语义、Pinax/Scaena/Eikona，不接真实云账号。
