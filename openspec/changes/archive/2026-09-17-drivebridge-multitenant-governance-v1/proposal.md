## Why

用户已批准单部署多租户、独立团队空间和管理员只读监督。当前 admin 密钥允许直接轮换他人凭据，不能作为新治理模型的授权来源。

## What Changes

- 增加统一身份、租户成员关系、团队与独立空间权限；显式部署侧激活和最高管理员引导。
- 增加管理读取、范围/清单审批、15 分钟应急、站内通知、访问审计和本人领取的人工恢复。
- 保留旧数据和引用；激活后旧 admin 不自动扩权，禁止旧轮换接口绕过本人恢复。

## Capabilities

### New Capabilities
- `tenant-governance`: 文件服务多租户治理、权限、通知与恢复。

### Modified Capabilities
- `drivebridge-personal-service`: 显式激活后的租户边界、角色投影与在线密钥恢复安全收紧。

## Impact

服务端与 SDK、Web、远程 CLI/MCP。跨用户读取和应急均要求持久审计；不部署、不接真实账号、不新增组织计费平台。
