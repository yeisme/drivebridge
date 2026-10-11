## 1. 合同实现

- [x] 1.1 purpose 白名单与同字段复用：`CreateHandoffBinding` 接受 `reference|vault|notes-attachments|materials|saveback|owner-store`，未知 `invalid_binding`；未 disable 的相同 owner+agent+space+prefix+consumer+project+purpose 复用。Owner: implementer。范围：`internal/service/handoffs.go`、`sdk/handoffs.go` 加法常量。验证：service 单测 Purpose/Binding。 | evidence: `TestHandoffBindingPurposeAllowlist`、`TestHandoffBindingCreateReusesSameScope`
- [x] 1.2 当前路径复核：`enforceBindingPrefix` 供 `GetHandoff`、`BindReference`、`readableHandoff` 使用；`OpenHandoff` 在 Download 后再查一次。越界 `forbidden`。Owner: implementer。范围：`internal/service/handoffs.go`。验证：Prefix 单测。 | evidence: `TestHandoffPrefixRecheckOnRead`
- [x] 1.3 disable 停新读：保持 disable 后 BindReference 与 OpenHandoff 失败；源文件仍在。Owner: implementer。范围：`internal/service/handoffs.go`、`handoffs_test.go`。验证：Disable 单测。 | evidence: `TestHandoffBindingDisableStopsReads`
- [x] 1.4 标签非认证：Agent 不能 CreateHandoffBinding；错误 Agent 即使 consumer 相同也 forbidden。Owner: implementer。范围：`handoffs.go`、测试。验证：Caller/Agent 单测。 | evidence: `TestHandoffCallerLabelsAreNotAuth`

## 2. CLI、文档与校验

- [x] 2.1 CLI `--purpose` help 列出允许值，默认仍 `reference`。Owner: implementer。范围：`cmd/drivebridge-server/handoffs.go`。验证：构建与 help 文本。
- [x] 2.2 `docs/creative-resource-handoff.md` 增量说明 purpose 白名单、读取前缀复核、disable 不停删副本。Owner: implementer。
- [x] 2.3 聚焦测试 + `openspec validate drivebridge-server-bind-prefix-v1 --strict --no-interactive` + `CGO_ENABLED=0 go build ./cmd/drivebridge-server`。验证命令：`python3 scripts/check.py -- go test ./internal/service ./sdk -count=1 -run 'Handoff|Binding|Prefix|Purpose'`。 | evidence: `temp/integration-test-runs/20260919T164543Z-0f01dcd2`（passed）；strict valid；CGO build exit 0
