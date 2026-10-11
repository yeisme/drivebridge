## Why

Key 管理缺少删除；回收站不能批量操作，也看不到自动清理。服务端合同已补齐，网页需要消费。

## What Changes

- Key 行提供吊销/删除（`can_revoke`/`can_delete`），确认后调用 key-directory revoke/delete。
- 回收站：多选、恢复所选、永久删除所选、空间保留期选择、显示清理日期。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-web`：Key 删除与回收站批量/保留期。

## Impact

split-owner：状态仍在 backend-server/drivebridge。不新增治理能力。
