# DriveBridge 文档

本项目拥有 OneDrive 文件传输和稳定引用；Scaena 拥有生成任务。运行入口和限制见 [README](../README.md)。实现与验收记录在 `openspec/changes/archive/`：

- `drivebridge-local-mcp-v1` / `drivebridge-host-handoff-v1` / `drivebridge-remote-client-v1`（2026-09-11 归档）
- `drivebridge-activation-readiness-v1`（2026-09-11 归档：doctor 预检与首用 runbook）

Platform v2 设计（active，实现归后续波）：`openspec/changes/` 下 `drivebridge-seamless-onedrive-auth-v1`（device-code 登录+drive/root 自动发现）、`drivebridge-generic-backends-v1`（任意 rclone 后端传输面+软链接策略）、`drivebridge-bisync-v1`（受守卫双向同步，keep-both）。

- [首次真实使用 runbook](activation-local.md)
- [本地验证记录](verification.md)
- [远程模式验证](remote-verification.md)
