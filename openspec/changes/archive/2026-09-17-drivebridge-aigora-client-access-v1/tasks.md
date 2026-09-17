# 实施与验收任务

本 change 已将 design 承诺落成 proposal 与 specs 增量，任务引用 design.md 已承诺的能力，不虚构 spec 增量。状态由 scripts/openspec-tasks 维护。

Lane 说明：
- lane A（实现）：internal/remote、internal/config、cmd/drivebridge 的代码改动，任务间按依赖串行。
- lane V（验证）：evidence runner、race、旧合同回归，依赖 lane A 全部完成。
- lane D（文档/收尾）：docs 与 strict validation，依赖 lane V。

## 1. Aigora 凭证交换适配（lane A）

- [x] 1.1 显式 `aigora_url` 才启用 SDK 短期凭证交换；owner: mcp/drivebridge；范围: `internal/remote/remote.go`（`NewWithAigora` / `sdk.NewAigora`）、`internal/config/config.go` 允许键清单；依赖: 无；对应 design「只有显式配置 `aigora_url` 才启用」。
  - 验证：`python3 scripts/check.py -- go test ./internal/remote ./internal/config -count=1`
  - 预期：未配置 `aigora_url` 时 Adapter 行为与原 domain credential 模式完全一致；配置后走 `aigora_key_exchange`。
  - 失败复查：先读 `internal/remote/access_contract_test.go` 失败断言，确认是配置键未被允许键清单接受还是 SDK 构造分支错误，不改旧模式测试。
  - evidence: 20260917T142335Z-42c282a4；`TestAigoraLegacyModeUnchangedWithoutExplicitURL` 断言未配置时直连静态 token 且无交换调用。
- [x] 1.2 Key 隔离与失败语义；owner: mcp/drivebridge；范围: `internal/remote/remote.go`（`remote_token_file` 读取路径、HTTP client 重定向策略）、`internal/config/config.go`；依赖: 1.1；对应 design「Key 不发往 DriveBridge、不跟随重定向、失败不回退 legacy/BYOK」。
  - 验证：`python3 scripts/check.py -- go test ./internal/remote -count=1 -run 'Aigora|Access'`
  - 预期：fake 服务断言请求体/Header 不含长期 Key；重定向响应报错不跟随；交换失败返回原错误且无静默回退。
  - 失败复查：用 httptest 记录真实出站请求，核对是否把 Key 发往非 Aigora origin；不得为通过测试放宽重定向限制。
  - evidence: 20260917T142336Z-935d08ad；`TestAigoraKeyNeverSentToFileService`、`TestAigoraRedirectAndExchangeFailureDoNotFallBack` 通过。

## 2. 准入停止与恢复合同（lane A）

- [x] 2.1 `service_admission_required` 停止语义；owner: mcp/drivebridge；范围: `internal/remote/remote.go`、`cmd/drivebridge/remote_transfer.go` 错误映射；依赖: 1.1；对应 design「返回 `service_admission_required` 时必须停止，不能替换 Key 或另走直连」。
  - 验证：`python3 scripts/check.py -- go test ./internal/remote ./cmd/drivebridge -count=1`
  - 预期：CLI/MCP 以稳定错误码呈现并停止，输出不含替换凭据或直连建议。
  - 失败复查：对照 `docs/error-codes.md` 核对错误码拼写与退出路径。
  - evidence: 20260917T142354Z-99c5545f；`TestAigoraAdmissionRequiredStopsWithStableCode` 断言 CLI/MCP 均透传该码且不含 direct/byok/swap/replace 建议；`internal/recovery` 注册表与 `docs/error-codes.md` 新增该码。
- [x] 2.2 恢复原 operation ID 与 idempotency key；owner: mcp/drivebridge；范围: `internal/remote/remote.go` 恢复分支、`internal/bridge/model.go` 操作记录读取；依赖: 2.1；对应 design「恢复时使用原 operation ID 和 idempotency key；不自动创建新的付费任务」。
  - 验证：`python3 scripts/check.py -- go test ./internal/remote ./internal/bridge -count=1`
  - 预期：未知结果按原标识对账，重试不产生第二个远程操作。
  - 失败复查：检查 fake 服务侧操作计数，确认无新建付费任务调用。
  - evidence: 20260917T142403Z-c16cd030；`TestAigoraResumeUsesOriginalOperationIdentifiers` 断言 resume 命中 `/api/v1/operations/op-1/resume` 且 `/api/v1/uploads` 只被调用一次。

## 3. 客户端发现资源（lane A，与分组 2 可并行）

- [x] 3.1 `drivebridge://service-access` 无需网络可读资源；owner: mcp/drivebridge；范围: `internal/bridge/mcp.go` 资源注册、`internal/remote/remote.go` 认证模式投影；依赖: 1.1；对应 design「说明当前认证模式、工具 schema 发现、适配器文件根范围、HTTP 传输责任、授权与消费确认、未知结果对账方式」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge -count=1 -run 'MCP|Resource'`
  - 预期：资源在离线环境可读，文本覆盖 design 列举的六项说明，不含 token、临时 URL 或本地凭据路径明文。
  - 失败复查：grep 资源输出确认无秘密字段；确认未把「工具列出」描述成「当前身份有写权限」。
  - evidence: 20260917T142420Z-327c27bd；`TestAccessResourceAvailableWithoutNetworkOrInstalledCLI` 离线读取通过。

## 4. 回归与 race（lane V）

- [x] 4.1 remote、config、CLI 全量回归与 race；owner: mcp/drivebridge；范围: `internal/remote`、`internal/config`、`cmd/drivebridge`；依赖: 1.1–3.1；对应 design「remote、config、CLI 回归与 race 通过」。
  - 验证：`python3 scripts/check.py -- go test ./internal/remote ./internal/config ./cmd/drivebridge -count=1` 与 `python3 scripts/check.py -- go test -race ./internal/remote ./cmd/drivebridge -count=1`
  - 预期：全部通过；证据六件套（summary.json、command.txt、stdout.log、stderr.log、env.json、artifacts/）写入 `temp/integration-test-runs/<run-id>/`，失败保持原退出码且脱敏。
  - 失败复查：确认失败为本次引入还是既有并发漂移；只修复本 change 引入的失败，保留无关脏工作区改动。
  - evidence: 20260917T142439Z-6c51b865 回归通过；20260917T142447Z-5f00ff99 race 通过；两 run 均含六件套且退出码 0。

## 5. 收尾质量门（lane D）

- [x] 5.1 纯 Go 构建、vet 与文档核对；owner: mcp/drivebridge；范围: 全仓、`docs/remote-verification.md`（认证模式与恢复说明段落）；依赖: 4.1。
  - 验证：`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、`openspec validate drivebridge-aigora-client-access-v1 --strict --no-interactive`
  - 预期：三项全过；文档区分本机凭证交换与远程服务责任，不宣称真实云账号、远程服务或生产部署已验收。
  - 失败复查：strict validation 若因缺 proposal/specs 报错，先补规格成稿再 archive，不得绕过校验。
  - evidence: vet 20260917T142534Z-ecbe9456、CGO_ENABLED=0 build 20260917T142535Z-d01315a8 通过；strict validation 通过；docs/remote-verification.md 新增「认证模式与恢复」段落并明确无真实云账号/生产验收。

验证证据：`temp/integration-test-runs/20260917T142447Z-5f00ff99/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
