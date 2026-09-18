# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。

- [x] 1.1 依赖无；串行 A。PersistentPreRunE 派生 command identity + 覆盖表；mount status/stop action 归一；storage.go 手工赋值删除。验收：`go test ./cmd/drivebridge -run TestCLI -count=1` 全绿且 storage.txt 失败信封断言（含 space-add/storage-add）过。 | evidence: main.go commandAction；storage.txt 24-27。
- [x] 1.2 依赖 1.1；串行 A。deviceauth 两处 SaveConfig→Data().Save() 传播；README 快速入门/命令树行修正。验收：`go test ./internal/bridge -count=1`；README 行已更新。 | evidence: deviceauth.go 两处 config_write_failed；README 27/32/64 行。
- [x] 1.3 依赖 1.2；串行 V。`python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、本包 strict 校验。 | evidence: 见最终复跑记录。
