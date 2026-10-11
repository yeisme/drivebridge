## Why

多租户=分片部署已落地，但每个新租户部署的启动摩擦集中在两处：①`serve --enabled` 硬性要求 OIDC issuer，key 化部署无法上线；②用户创建只有 OIDC 登录一条路，无法批量开通。运营方需要的是管理员/Agent 拿着密钥清单直接批量建用户——不支持 OAuth、不自助注册、第一版由管理员或 Agent 签发 key 并导入数据库。

## What Changes

- 新增**操作员签发用户密钥**：本地 CLI `users import` 从 stdin JSON 批量建用户（条目可带操作员自有 key 或由系统生成 `dbu_` 密钥）；明文只在签发输出出现一次，库内只存 SHA-256；`users list`/`users revoke` 管理生命周期。
- 新增 **key-only 部署模式**：`serve --enabled` 在未配置 issuer 时不再拒绝启动；浏览器登录如实报 `oidc_not_configured`，Bearer 用户密钥照常认证。
- 新增 **admin 密钥 HTTP 开面**：持有 admin 用户密钥的调用方可 `POST /api/v1/users` 批量建用户（与 CLI 同校验）；普通用户密钥、Agent token、会话一律拒绝。
- 部署 runbook 新增"新租户四步"配方：config init → platform-connect（共享桶前缀作用域 key）→ users import → serve。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增操作员签发用户密钥、key-only 部署、admin 密钥开面三个需求；不删除 OIDC 登录路径（需要浏览器登录的部署照旧）。

## Impact

backend-server/drivebridge 拥有实现与验收；mcp/client 零变化（Bearer 认证通道已存在）。密钥不进配置文件/日志/回执；用户密钥身份与 OIDC 身份在同一 User 表共存（Issuer="key"），个人身份隔离语义不变。
