## Why

用户批准多租户与管理员只读管理，需要 Web 按服务端能力展示不同视图，并提供授权、通知与恢复闭环。

## What Changes

- 标签页内租户与空间上下文，切换时清空旧视图并拒绝陈旧响应。
- 管理中心覆盖团队、成员、权限、请求、审批、执行、应急、通知和审计。
- 新密钥由本人生成与领取，管理员只核验恢复请求；恢复码仅显示一次。

## Capabilities

### New Capabilities
- `tenant-governance-web`: 多租户管理与授权交互。

### Modified Capabilities

## Impact

仅 Web owner；HTTP 合同与权限权威在 backend-server/drivebridge。复用既有组件、Vitest 与 Playwright；不部署、不处理真实凭据。
