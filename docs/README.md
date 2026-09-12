# DriveBridge 文档

本项目拥有 OneDrive 文件传输和稳定引用；Scaena 拥有生成任务。运行入口和限制见 [README](../README.md)。实现与验收记录在 `openspec/changes/archive/`：

- `drivebridge-local-mcp-v1` / `drivebridge-host-handoff-v1` / `drivebridge-remote-client-v1`（2026-09-11 归档）
- `drivebridge-activation-readiness-v1` / `drivebridge-generic-backends-v1`（2026-09-11 归档）/ `drivebridge-cli-config-v1` / `drivebridge-agent-workspace-v1`（2026-09-12 归档：cobra+viper；工作区目录+CLI 传输+install 一键注册）

设计（active，实现归后续波）：`drivebridge-seamless-onedrive-auth-v1`（device-code 登录+自动发现）、`drivebridge-bisync-v1`（守卫双向同步）、`drivebridge-agent-discovery-v1`（search/changes 游标/内容寻址/TTL）、`drivebridge-soft-delete-v1`（软删除+恢复，永久删除归用户）。

- [首次真实使用 runbook](activation-local.md)
- [本地验证记录](verification.md)
- [远程模式验证](remote-verification.md)
