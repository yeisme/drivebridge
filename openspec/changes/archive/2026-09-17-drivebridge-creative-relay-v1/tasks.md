# 实施与验收任务

状态由 scripts/openspec-tasks 维护。全部任务已完成；原始验收证据见各任务 evidence 与文末 run 记录。

Lane 说明：
- lane A（实现）：internal/bridge、internal/remote、internal/config、cmd/drivebridge 的串行实现链，1.1→1.4 依次依赖。
- lane V（验证）：race、进程恢复与旧合同回归，依赖 lane A 全部完成。
- lane D（文档/收尾）：docs、CLI help 与 strict validation，依赖 lane V。

## 1. 目标绑定配置与 Relay service 骨架（lane A）

- [x] 1.1 建立受限目标绑定配置和 Relay service 接口/操作关联，保留原 state 锁；owner: mcp/drivebridge；范围: `internal/config/config.go`（允许键清单）、`internal/bridge/model.go`（Operation/关联模型）、`internal/bridge/relay.go`；依赖: 无；对应 requirement「绑定目标与固定来源」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./internal/config ./cmd/drivebridge -count=1`
  - 预期：覆盖配置创建、未知键拒绝和未授权目标拒绝；未知键不能静默启用写能力。
  - 失败复查：区分配置并发漂移与逻辑错误；未知键失败先核对 config.go 允许键清单。
  - evidence: 20260916T190205Z-2160fb96 four-package gate green after the concurrent config drift cleared; relay target registration/conflict, unknown owner, unknown key and unauthorized target rejections covered.

## 2. 固定来源读取与三 owner 中转（lane A）

- [x] 1.2 复用本机允许根和已有 provider 读取固定来源，按三个 fake owner 原输入协议中转；owner: mcp/drivebridge；范围: `internal/bridge/relay.go`、`internal/bridge/local_files.go`、`internal/remote/remote.go`、`internal/bridge/relay_owners_test.go`；依赖: 1.1；对应 requirement「绑定目标与固定来源」「三 owner 原生输入完成」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./internal/config ./cmd/drivebridge -count=1`
  - 预期：同一 component 命令覆盖同机/不同 origin、generic 后端、大小/摘要校验和越界拒绝；两阶段握手（awaiting_source → ProvideImportSource → 原 input ID）元数据重放复用会话、冲突拒绝。
  - 失败复查：用 httptest fake owner 确认是元数据指纹不匹配还是传输层错误；不引入新后端或 Provider 调用。
  - evidence: 20260916T190205Z-2160fb96; three fake owners (Eikona 图片/Scaena 视频/Sonora 音频) on separate origins complete original inputs and replay without re-transfer; generic rclone backend source, size/digest and boundary rejections covered.

## 3. 恢复、独占写入与取消（lane A）

- [x] 1.3 实现原操作状态、单写者 lease、恢复和取消；owner: mcp/drivebridge；范围: `internal/bridge/relay.go`、`internal/bridge/model.go`、`internal/recovery/codes.go`、`cmd/drivebridge/recovery.go`；依赖: 1.2；对应 requirement「幂等恢复与独占写入」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./internal/config ./cmd/drivebridge -count=1`
  - 预期：覆盖进程退出、complete 丢响应、来源变化、输入到期；恢复先查目标状态，不重建输入，不产生第二个领域输入；取消不删除已完成资产。
  - 失败复查：真实子进程 kill 后在新 CLI 进程 resume，核对目标侧 import 计数恰好为 1。
  - evidence: 20260916T190205Z-2160fb96; real subprocess kill mid-transfer resumes in a fresh CLI process with exactly one import; lost completion, source change and expired input recovery covered without rebuilding inputs.

## 4. CLI/MCP 入口与能力诚实（lane A）

- [x] 1.4 接入 CLI/MCP 与精确能力发现，私下获取传输能力并投影安全回执；owner: mcp/drivebridge；范围: `cmd/drivebridge/relay.go`（relay_start/relay_cancel 等 Cobra 命令）、`internal/bridge/mcp.go`（registry）、`internal/bridge/transfer.go`；依赖: 1.3；对应 requirement「客户端能力诚实与秘密隔离」「回存和兼容」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./internal/config ./cmd/drivebridge -count=1`
  - 预期：覆盖 MCP-only 缺口报告（不假装上传客户端路径）、临时 grant 不进入持久回执或模型上下文、旧 upload/download/Scaena socket 行为不变。
  - 失败复查：grep 回执与 MCP 输出确认无临时 URL/grant；旧合同回归失败优先对照既有 golden。
  - evidence: relay_start/relay_cancel plus original operation_get and CLI; transient grant isolation and no duplicate transfer: 20260916T134736Z-f1ea3190. CLI/remote race passed in 20260916T134130Z-cb8876cf.

## 5. race、进程恢复与旧合同回归（lane V）

- [x] 1.5 全量 race 与进程级恢复验证；owner: mcp/drivebridge；范围: `internal/bridge`、`internal/remote`、`cmd/drivebridge`（含 testdata testscript）；依赖: 1.1–1.4；对应 requirement「幂等恢复与独占写入」「回存和兼容」。
  - 验证：`python3 scripts/check.py -- go test -race ./internal/bridge ./internal/remote ./cmd/drivebridge -count=1`
  - 预期：复用现有 testscript 验证进程恢复与并发；六件套证据齐全、退出码为 0、`-v` 输出确认每个新增 relay 用例实际执行。
  - 失败复查：核对 `temp/integration-test-runs/<run-id>/summary.json` 中新增用例计数，排除「测试被 skip 冒充通过」。
  - evidence: 20260916T190226Z-c273b45f race gate green; 20260916T190312Z-98f73e70 -v run shows every new relay case executed; six-piece evidence and exit code 0 verified.

## 6. 文档与收尾质量门（lane D）

- [x] 1.6 同步 docs、CLI help 与本 change strict validation；owner: mcp/drivebridge；范围: `docs/creative-resource-relay.md`、CLI help 文本、openspec 校验；依赖: 1.5。
  - 验证：`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、`openspec validate drivebridge-creative-relay-v1 --strict --no-interactive`
  - 预期：文档区分电脑离线等待和服务不可用；不报告真实部署完成；仅新增 nullable/default 字段，pure-Go/GORM；禁用 relay 不迁移引用或删除旧操作。
  - 失败复查：strict validation 失败先修 spec/tasks 格式，不绕过校验。
  - evidence: docs/creative-resource-relay.md and CLI help cover three owners; go vet ./..., CGO_ENABLED=0 build and strict validation passed; no deployment claimed.

验证证据：`temp/integration-test-runs/20260916T190312Z-98f73e70/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
