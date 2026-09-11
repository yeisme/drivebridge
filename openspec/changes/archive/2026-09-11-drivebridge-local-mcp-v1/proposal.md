## Why

本机 OneDrive 文件 MCP：用户需要 Agent 自动上传创作素材，并在调用火山前即时取得 OneDrive 下载授权。

## What Changes

实现固定版本 rclone、Graph、GORM 状态和 stdio 工具，所有写入口默认关闭。

## Capabilities

### New Capabilities
- `drivebridge-local-files`：本机 OneDrive 文件 MCP。

### Modified Capabilities
无已发布合同删除或改名。

## Impact

使用固定版本 rclone 作为 OAuth 刷新唯一权威；Graph 从同一配置借用 access token。工具注册表生成 schema。状态和 space 只通过 CLI/service 修改。
