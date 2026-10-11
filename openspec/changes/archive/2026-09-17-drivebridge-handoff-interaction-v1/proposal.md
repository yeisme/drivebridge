# 文件交接查询与恢复

## Why

底层传输已存在，但客户端仍需要猜 ID 或借用其他入口查询，导致原操作恢复困难。

## What Changes

为 input.bind_reference 增加可选 binding_id、consumer_request_ref 和 RFC3339 request_expires_at，保留旧必填项及返回字段。增加 input.handoff_status/renew/cancel 与 handoff status/renew/cancel CLI，复用既有 service/SDK。统一身份的状态查询属于只读；续期、取消及字节获取保留原准入。

## Capabilities

### New Capabilities
- `drivebridge-interaction`：增量发现与恢复体验。

### Modified Capabilities
无；现有有效请求继续支持。

## Impact

本 owner 的 CLI、MCP、测试、文档与规范 Skill，不改变线上配置或发布候选。
