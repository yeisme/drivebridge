# Tasks

- [x] 1.1 MCP `users_create`/`users_list`/`users_revoke` 工具：对象数组 schema、admin 门、哈希不出库、float64/int 双解码
- [x] 1.2 MCP 管理面测试：admin 创建/列出/吊销 + 普通密钥 forbidden 矩阵（适配 `Principal.Admin` 合并语义）
- [x] 1.3 SDK `UsersCreate`/`UsersList`/`UsersRevoke` 对接 `drivebridge-key-session-login-v1` 的 HTTP 面实测（`{"keys":[...]}` 与 path 参数吊销已对齐真 handler；`TestUsersManagementSDKAgainstHTTPSurface` 走真实 HTTP mux：批量签发、明文密钥只在签发响应出现一次、导入密钥即刻可用、非 admin 密钥 forbidden、吊销后下一请求 unauthorized、列表只含生命周期字段）

验证证据：MCP 面测试经 `go test ./internal/service/ -run TestUsersManagementAgentSurface` 全绿（合并态工作树，2026-09-14）。SDK e2e 经 `go test ./internal/service/ -run TestUsersManagementSDKAgainstHTTPSurface` 全绿；整树 `python3 scripts/check.py`（`go test -race ./...`，temp/integration-test-runs/20260914T184330Z-1976b340 passed）、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server` 全绿（合并态工作树，2026-09-14）。
