## Why

Key 管理已有复制、轮转、管理员吊销/归档，但本人无法删除多余或已失效 Key。回收站只能逐个恢复/永久删除，没有保留期和批量操作，文件会无限堆积。

## What Changes

- 本人可吊销、删除自己的 Key（需确认）；不能删掉最后一个有效 Key 或最后一个管理员 Key；删除会擦除密文并级联会话。
- 目录增加 `can_revoke` / `can_delete`。Agent 仍禁止。
- 回收站记录 `trashed_at`；空间默认 30 天自动永久删除；可设 1–365 天。
- `POST /api/v1/files/batch/restore` 与 `POST /api/v1/files/batch/purge`（确认头），最多 100 条，部分成功带失败码。
- 服务启动与每小时扫描过期回收站。MCP 不增加永久删除工具。

## Capabilities

### New Capabilities

- `key-self-delete`：本人吊销/删除登录 Key。
- `trash-retention-batch`：回收站保留期、定时清理、批量恢复/永久删除。

### Modified Capabilities

- `drivebridge-personal-service`：文件增加 `trashed_at`；空间增加 `trash_retention_days`。

## Impact

fit：密钥与回收站状态归本服务。网页为 split-owner。不改生成任务、不伪造直链、不让 Agent 永久删除。
