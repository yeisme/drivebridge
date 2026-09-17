# 实施与验收任务

状态由 scripts/openspec-tasks 维护。任务 1.1 已完成；验收证据见文末 run 记录。

Lane 说明：
- lane A（实现）：远端工具/SDK 的上下文增量，1.1 单任务。
- lane V（验证）：单元回归、真实远端流程与 evidence runner。
- lane D（文档/收尾）：能力资源与 docs、strict validation。

## 1. 可选租户/空间上下文与 SDK 隔离（lane A）

- [x] 1.1 远端工具公开租户和空间上下文并验证 SDK 隔离；owner: mcp/drivebridge；范围: `internal/remote/remote.go`（每次调用复制 Adapter、独立 Scoped SDK client）、`cmd/drivebridge/remote_transfer.go` 与 `cmd/drivebridge/commands.go`（可选 `tenant_context`/`space_context` 参数）；依赖: 无；对应 requirement「Context is not authority」。
  - 验证：`python3 scripts/check.py -- go test ./internal/remote ./cmd/drivebridge -count=1`
  - 预期：两次并发调用选择不同租户时使用独立 SDK client 上下文，不修改共享 Adapter；未提供上下文时保留原行为；仅新增可选字段，SDK 旧构造器继续工作；Agent token 由服务端推导原租户与空间，不能越界。
  - 失败复查：并发污染先检查是否误改共享 Adapter；上下文缺失回归对照原有远端流程测试。
  - evidence: `temp/integration-test-runs/20260916T111031Z-5674731b/summary.json`.

## 2. 发现诚实与恢复入口（lane A，可与 1.1 并行，本轮随 1.1 一并交付）

- [x] 1.2 tools/list 与能力资源解释上下文边界；owner: mcp/drivebridge；范围: `internal/bridge/mcp.go`（tools/list inputSchema 与 capabilities 资源）；依赖: 无（实现上与 1.1 同窗交付）；对应 requirement「Context is not authority」的「Client without an installed CLI」场景。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge -count=1`
  - 预期：未安装产品 CLI 的 MCP 客户端可从上文资源获得上下文参数说明、授权失败恢复入口（服务端 Web 人工审批，不使用 Agent 自批）与本机文件访问边界；上下文不授予权限。
  - 失败复查：确认能力文本未把上下文参数描述为权限来源；恢复入口指向服务端 Web 而非 Agent 通道。
  - evidence: `temp/integration-test-runs/20260916T111031Z-5674731b/summary.json`.

## 3. 收尾质量门（lane D）

- [x] 1.3 最终校验与文档核对；owner: mcp/drivebridge；范围: `docs/tenant-context.md`、全仓质量门；依赖: 1.1–1.2。
  - 验证：`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、`openspec validate drivebridge-tenant-context-v1 --strict --no-interactive`
  - 预期：三项全过；文档明确不改变本机空间模型、不增加审批或应急的 Agent 权限；已有单元测试与真实远端流程继续由 evidence runner 覆盖。
  - 失败复查：strict validation 失败先修 spec/tasks 对应关系，不绕过校验。
  - evidence: `temp/integration-test-runs/20260916T111031Z-5674731b/summary.json`.

验证证据：`temp/integration-test-runs/20260916T111031Z-5674731b/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
