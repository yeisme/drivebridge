## Why

review 波修三 bug：①`ListAgents` 按不存在的 `expires_at` 列排序（Agent.Expires 实际列名 `expires`），错误被吞、列表恒空（当前无调用方，死代码带病）；②`RotateUserKey` 允许轮换已过期 key——新 key 继承过去时刻的过期时间，出生即死；③运行中部署轮换 key 需停服窗口（CLI 与 serve 互斥 state 锁，开箱验收实测）——补 HTTP rotate 端点。

## What Changes

- `ListAgents` 改按真实列 `expires` 排序。
- `RotateUserKey` 拒绝已过期/已吊销 key（not_found）。
- 新增 `POST /api/v1/users/{id}/rotate`（admin；body days 可选）——与 revoke 同范式，明文一次返回。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：Key rotation 需求补"过期不可轮换"与 HTTP 端点场景。

## Impact

纯修复与增量端点；零迁移（不改列名标签，避免旧库数据滞留）。
