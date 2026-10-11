## Why

用户密钥管理此前只有操作员 CLI 与 admin HTTP 创建接口；Agent/SDK 消费面没有 users 管理入口，无法把"新租户四步"中的用户签发交给人以外的调用方。

## What Changes

- MCP 新增 `users_create`（批量条目对象数组：name/key/days/admin）、`users_list`、`users_revoke` 三个工具，admin 门与 HTTP 完全一致（MCP 认证面是 Bearer 用户密钥）。
- SDK 新增 `UsersCreate`/`UsersList`/`UsersRevoke` 方法与 `UserImport`/`UserKeyIssued`/`UserKey` 类型。
- 列出面永不返回哈希；创建的明文密钥只在签发响应出现一次；吊销按 id 生效。

## 撞车与收敛（2026-09-14）

本 change 与并行 lane 的 `drivebridge-key-session-login-v1` 在 users HTTP 面（list/revoke）撞车。判定：**HTTP 面归 key-session-login change**（对齐 agents 范式：`GET /api/v1/users` 返回 `{"keys":[...]}`、`POST /api/v1/users/{id}/revoke` path 参数、`Principal.Admin` 统一门）；**本 change 只拥有 MCP 工具与 SDK**，SDK 端点形态已对齐该判定。两 lane 在同一工作树收敛后由 key-session-login 波统一提交。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-service`：Admin-key user provisioning over HTTP 需求扩展覆盖 MCP 工具与 SDK 的创建/列出/吊销。

## Impact

backend-server/drivebridge；HTTP 面语义由 `drivebridge-key-session-login-v1` 的 ADDED 需求承载；CLI 与既有 HTTP 创建语义不变。
