## 1. 合同与实现

- [x] 1.1 CopyFile + HTTP `POST /api/v1/files/{id}/copy` + SDK `Copy`；CAS、rename/fail、幂等、resume、权限
  - owner: backend-server/drivebridge
  - 范围：`internal/service/archive.go`、`files.go`、`browser.go`、`http.go`、`mcp.go` resume、`sdk/client.go`、`error_messages.go`
  - 依赖：无
- [x] 1.2 测试：同空间秒传、冲突改名、fail、回收站源、version_conflict、非受管目标、幂等重放
  - 验证：`go test ./internal/service -count=1 -run 'TestCopy'`
- [x] 1.3 文档：`docs/media-sharing.md` 增加 copy 合同
  - 验证：对照 spec 字段与错误码

## 2. 收口

- [x] 2.1 `python3 scripts/check.py` 中相关包测试 + `go vet ./...` + `CGO_ENABLED=0 go build ./cmd/drivebridge-server`

验证证据：`temp/integration-test-runs/20260919T110717Z-d66d1765/summary.json`（相对 `backend-server/drivebridge`）。
