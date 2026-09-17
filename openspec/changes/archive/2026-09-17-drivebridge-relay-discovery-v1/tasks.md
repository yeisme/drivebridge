# 实施与验收任务

状态由 scripts/openspec-tasks 维护。全部任务已完成；验收证据见文末 run 记录。

Lane 说明：
- lane A（实现）：CLI/MCP 只读 relay_targets 增量，1.1 单任务。
- lane V（验证）：真实入口回归、race 与证据。
- lane D（文档/收尾）：本地文档、Skill/profile 与 strict validation。

## 1. 只读 relay_targets 增量（lane A）

- [x] 1.1 实现增量交互合同并保持既有权限边界；owner: mcp/drivebridge；范围: `cmd/drivebridge/relay.go`（CLI targets 输出）、`internal/bridge/mcp.go`（relay_targets 只读入口）、`internal/bridge/relay.go`（RelayTarget 安全投影）；依赖: 无；对应 requirement「交互保留原操作与授权边界」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./cmd/drivebridge -count=1`
  - 预期：返回 ID、源空间、项目、disabled、not_probed；隐藏端点和凭据文件路径；原 targets 数组输出不变；CLI 明确必要配置选项。
  - 失败复查：输出脱敏失败先 grep 响应中的 endpoint/credential 路径；原数组回归失败对照既有 golden。
  - evidence: temp/integration-test-runs/20260916T164029Z-672c7a70 full race passed; safe relay target discovery test, vet/build and target Skill/profile validation passed.

## 2. 真实入口验证与恢复边界（lane V）

- [x] 1.2 验证真实入口、错误恢复与安全发现；owner: mcp/drivebridge；范围: `cmd/drivebridge`（真实 CLI 请求）、`internal/bridge`（MCP 分派、原标识保持、终态、权限拒绝）；依赖: 1.1；对应 requirement「交互保留原操作与授权边界」的「先发现后操作」「恢复原操作」场景。
  - 验证：`python3 scripts/check.py -- go test -race ./...`
  - 预期：覆盖真实 CLI 请求、MCP 分派、原标识保持、终态、权限拒绝与输出脱敏；不把 not_probed 宣称为 ready；恢复经原 owner 标识，不创建替代生成、不复活已终结输入；证据写回 `temp/integration-test-runs/<run-id>/` 六件套。
  - 失败复查：先查原操作记录，不改身份、目的地或幂等键；失败归因本次引入/既有漂移后再修。
  - evidence: temp/integration-test-runs/20260916T164029Z-672c7a70 full race passed.

## 3. 文档、Skill 与收尾质量门（lane D）

- [x] 1.3 同步本地文档与规范 Skill，执行最终校验；owner: mcp/drivebridge；范围: `docs/creative-resource-relay.md`（发现入口段落）、目标 Skill/profile；依赖: 1.2。
  - 验证：`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、`openspec validate drivebridge-relay-discovery-v1 --strict --no-interactive`
  - 预期：仅新增可选字段与动作，原数组、URI、命令和权限不变；可停用新增入口而不删除操作记录；独立 v0.8.1 候选保持冻结，不触碰线上配置或发布候选。
  - 失败复查：strict validation 失败先修 spec/tasks 对应关系，不绕过校验。
  - evidence: temp/integration-test-runs/20260916T164029Z-672c7a70 vet/build and target Skill/profile validation passed.

验证证据：`temp/integration-test-runs/20260916T164029Z-672c7a70/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
