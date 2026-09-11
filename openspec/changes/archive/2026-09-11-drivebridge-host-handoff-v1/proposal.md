## Why

本机宿主安全交接与回存：用户需要 Agent 自动上传创作素材，并在调用火山前即时取得 OneDrive 下载授权。

## What Changes

通过私有 Unix socket 交接参考 URL；通过 Scaena artifact grant 回存成品。

## Capabilities

### New Capabilities
- `drivebridge-private-handoff`：本机宿主安全交接与回存。

### Modified Capabilities
无已发布合同删除或改名。

## Impact

宿主适配器属于 DriveBridge 发布包，代码为 internal/bridge/handoff.go、host_scaena.go；不引入 UI 或媒体代理。Scaena 保存领域任务，DriveBridge 保存独立 transfer。
