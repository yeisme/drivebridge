## Why

Agent 上传与导入的产物只散在传输记录里，用户需要一个"结果"视角挑选固定版本收进自己的空间；backend 已交付 `GET /api/v1/inbox` 与 `POST /api/v1/archive` 契约，client 缺消费面。

## What Changes

新增「产物收件箱」导航页：轮询 `/inbox`（4s，与传输中心同节奏），按完成/失败状态展示产物卡（来源 caller、任务 ref、错误码文案）；完成项可进入既有版本选择流；版本面板新增归档表单（目标空间/路径，`POST /archive`），`invalid_archive`/`unsupported` 错误码有中文文案。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-web`：新增产物收件箱与归档表单需求。

### New Capabilities

无。

## Impact

client/drivebridge-client 本地实现；数据与语义归 backend owner change `drivebridge-agent-artifact-archive-v1`。不改既有导航与传输契约。
