## Why

用户需要解释容量、传输与 Agent 行为，并能定位操作和安全问题。现有文件流程缺少统一统计口径与可查询审计。

## What Changes

增加本 owner 的资源统计、预算扫描、审计合同及四视图接入；保留原文件接口，以增量表和可选入口扩展。

## Capabilities

### New Capabilities
- `insights-audit`: 具有明确口径、权限隔离和故障策略的用量与审计。

### Modified Capabilities

无既有能力删除。

## Impact

client/drivebridge-client 拥有本地实现与验收。统计引擎归 backend SDK；本机采集归 MCP owner；网页展示归 client，判定为 split-owner。文件业务归原 owner，不新增主壳或租户平台。
