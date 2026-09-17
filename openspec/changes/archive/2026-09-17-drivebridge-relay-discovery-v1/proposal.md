# 本机中转目标发现

## Why

底层传输已存在，但客户端仍需要猜 ID 或借用其他入口查询，导致原操作恢复困难。

## What Changes

新增只读 relay_targets，使用现有 RelayTarget 安全投影。返回 ID、源空间、项目、disabled 和 not_probed；隐藏端点和凭据文件路径。CLI 明确必要配置选项，原 targets 数组输出不变。

## Capabilities

### New Capabilities
- `drivebridge-interaction`：增量发现与恢复体验。

### Modified Capabilities
无；现有有效请求继续支持。

## Impact

本 owner 的 CLI、MCP、测试、文档与规范 Skill，不改变线上配置或发布候选。
