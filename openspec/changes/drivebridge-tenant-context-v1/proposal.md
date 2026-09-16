## Why

服务端新增多租户治理，远端 CLI/MCP 需要显式选择租户与空间，同时保持原有 Agent 凭据的限定范围。

## What Changes

- 远端工具增加可选 tenant_context 与 space_context 参数，通过独立 SDK Scoped client 发送。
- tools/list 与能力资源解释上下文不授予权限、人工审批入口以及文件传输边界。

## Capabilities

### New Capabilities
- `remote-tenant-context`: 远端多租户上下文。

### Modified Capabilities

## Impact

仅远端适配器与公共 SDK，不改变本机空间模型、不增加审批或应急的 Agent 权限。
