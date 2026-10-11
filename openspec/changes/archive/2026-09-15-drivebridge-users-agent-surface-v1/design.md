# Design: drivebridge-users-agent-surface-v1

## 决策

1. **范围切分**：与 `drivebridge-key-session-login-v1` 撞车后，HTTP list/revoke 端点归该 change（agents 范式 path 参数、`Principal.Admin` 门）；本 change 只交付 MCP 工具与 SDK，避免双份端点。
2. **MCP 权限口径**：MCP 认证面是 Bearer 用户密钥，工具守卫用 `p.Key == nil || !p.Key.Admin`；`users_create` 走 `CreateUserUsers`（该函数在 key-session-login 落地后改认 `p.Admin`，两口径对 admin 密钥等价）。
3. **明文密钥只出现一次**：`users_create` 的 MCP 结果与 HTTP/CLI 一样携带一次性明文；`users_list` 只暴露生命周期元数据（id/name/admin/created/expires/revoked），哈希永不离开存储。
4. **MCP 参数形态**：批量条目用对象数组（`WithArray`+`Items` schema），解码同时接受 float64 与 int 数字（MCP JSON 路径是 float64，进程内测试是 int）。
5. **SDK 端点形态**：`UsersList` → `GET /api/v1/users` 解 `{"keys":[...]}`（行含 `user` 属主字段）；`UsersRevoke` → `POST /api/v1/users/{id}/revoke`。依赖 key-session-login 的 HTTP 面落地后才有真实端点，属前向对接。
