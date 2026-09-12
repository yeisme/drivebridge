# DriveBridge 文档

本项目拥有 OneDrive 文件传输和稳定引用；Scaena 拥有生成任务。运行入口和限制见 [README](../README.md)。实现与验收记录在 `openspec/changes/archive/`：

- `drivebridge-local-mcp-v1` / `drivebridge-host-handoff-v1` / `drivebridge-remote-client-v1`（2026-09-11 归档）
- 2026-09-11/12 归档：activation-readiness、generic-backends、cli-config（cobra+viper）、agent-workspace（目录+CLI+install）、soft-delete（软删除+恢复，永久删除归用户）

active change：`drivebridge-seamless-onedrive-auth-v1`（device-code 登录+自动发现）、`drivebridge-bisync-v1`（守卫双向同步）、`drivebridge-agent-discovery-v1`（批次一 search+changes 已实现；批次二内容寻址/TTL/gc/ping 待实现）。

- [首次真实使用 runbook](activation-local.md)
- [本地验证记录](verification.md)
- [远程模式验证](remote-verification.md)
