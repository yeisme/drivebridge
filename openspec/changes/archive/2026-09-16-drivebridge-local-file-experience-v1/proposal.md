## Why

本机视图需要真实目录、媒体预览、授权文件操作和远程副本交接，继续沿用现有 CLI/MCP owner。

## What Changes

扩展私有 socket 与回环 view，复用网页组件和共享媒体转换器；增加远程副本及创作输入交接，旧 MCP 文件数组不改变。

## Capabilities

### New Capabilities
- `local-file-experience`: 私有本机文件体验与远程副本。

## Impact

代码与状态归 mcp/drivebridge；公开分享只在远程 DriveBridge Server 创建，不把本机源文件变成公共服务。运行入口仍需要显式 enabled。
