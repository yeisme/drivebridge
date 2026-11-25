# drivebridge-agent-mcp-attribution-projection-v1 Tasks

- [x] 1.1 更新 `docs/server/personal-agent-loop.md` 记录 `/api/v1/operations` 的 `actor_type` 与 `origin_refs` 增量字段及无路径安全边界。
  - 验证：`openspec validate drivebridge-agent-mcp-attribution-projection-v1 --strict --no-interactive`
- [ ] 1.2 在根模块 MCP 与 `server/` 操作模型中记录可选创建期 `actor_type` 与 `origin_refs`（`gateway_call_ref`、`mcp_session_ref`）。
  - 验证：`go test ./... -count=1`
- [ ] 1.3 在 `GET /api/v1/operations` 投影中输出脱敏 `actor_type` 与 `origin_refs`，验证跨空间隔离与 `CGO_ENABLED=0` 构建。
  - 验证：`go vet ./... && CGO_ENABLED=0 go build ./cmd/drivebridge`
