## Why

当 Agent 经个人 MCP Gateway 或原生 MCP 工具调用 DriveBridge 执行参考素材拉取、成品回存或空间传输时，控制中心通过 `/api/v1/operations` 观察开放工具运行记录，需要准确获知该传输由类型化 `Agent` 发起并能关联对应的网关 `gateway_call_ref`，同时继续严守不暴露本地路径、文件名或临时下载链接。

## What Changes

- 在 DriveBridge MCP 与 HTTP 传输创建入口安全解析可选只读关联引用（`X-Gateway-Call-Ref`、`X-Gateway-Session-Ref`、`_meta["io.yeisme.gateway/correlation"]`），随运行操作记录保存。
- 在 `GET /api/v1/operations` 返回的操作摘要中增量附带可选 `actor_type`（`human | agent | service | unknown`，当由 Agent 令牌或 MCP 工具发起时为 `"agent"`）与安全 `origin_refs`（`gateway_call_ref`、`mcp_session_ref`）。

## Capabilities

### Modified Capabilities
- `drivebridge-personal-agent-loop`: 在个人文件传输 `/api/v1/operations` 投影中保留既有权限与文件身份的同时，增量暴露无路径的 `actor_type` 与 `origin_refs`。

## Impact

同仓 `cli/drivebridge`（根模块 MCP + `server/` HTTP/GORM）增量扩展，保持 `CGO_ENABLED=0` 构建，不暴露路径/文件名/临时 URL，不运行 `-race`。
