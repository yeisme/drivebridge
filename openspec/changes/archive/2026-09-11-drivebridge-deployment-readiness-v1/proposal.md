## Why

DriveBridge Server v1 是本地候选：生产部署步骤（OIDC 应用注册、TLS origin、platform-connect、备份演练）散落在 README，部署前/故障时无法一条命令确认 state 一致性与配置就绪。

## What Changes

新增 `doctor` 只读预检子命令与 `docs/deployment.md` 生产部署 runbook。不新增业务入口、不修改启动/备份合同、不做真实 OIDC/微软/S3 账号操作。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增 Deployment preflight 需求（只读报告、恢复导向失败、issuer 探测不中断其余检查）。

## Impact

doctor 不取实例锁、不调用 `Open`（避免把进行中操作改标 interrupted），以只读方式检查 state 三件一致性、web 构建产物、平台存储配置与可选 issuer discovery。真实 OIDC/S3/微软注册与公网部署仍由操作者独立执行验收。
