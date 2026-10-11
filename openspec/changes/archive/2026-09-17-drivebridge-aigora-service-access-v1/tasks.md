## 1. 实施与验证

- [x] 1.1 实现 EdDSA 服务凭证验证（固定 issuer/JWKS/Identity backend，audience 固定 `drivebridge`，有效期 ≤5 分钟，不授予上游角色外管理权） | owner: `internal/service/service_identity.go`、`internal/service/model.go`、`internal/config` | 依赖： 无；Lane A | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'ServiceIdentity' -count=1`；预期： 错误受众/超窗签发被拒且不产生本地授权；失败复查： 先跑 `-race`，再核对 issuer/audience 常量与测试时钟。
- [x] 1.2 新增 GORM 表 `ServiceIdentityBinding`（仅追加迁移）与 issuer+subject+身份组织三元组精确映射，绑定既有本地用户/租户/受限 Agent，不按邮箱合并，同映射不可改指 | owner: `internal/service/model.go`、`internal/service/service_identity.go` | 依赖： 1.1；Lane A | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： 迁移后旧部署默认关闭且旧用户可登录，邮箱相同但三元组不同不合并；失败复查： 检查迁移幂等性与既有 fixture 用户。
- [x] 1.3 切换（cutover）语义： 切换前校验租户全部活跃成员已映射，切换后旧用户 Key/会话/Agent 以 `unified_login_required` 拒绝，不通过清除单个映射恢复旧入口 | owner: `internal/service/service_identity.go`、`internal/service/auth.go`、`internal/service/error_messages.go` | 依赖： 1.2；Lane A | 验证： 同上命令覆盖切换前后矩阵；预期： 切换后旧凭据全部拒绝且错误码正确；失败复查： 区分映射缺失（`identity_mapping_required`）与已切换（`unified_login_required`）两条路径。

## 2. 实施与验证

- [x] 2.1 新增 GORM 表 `ServiceAdmission` 台账与托管上传边界的 reserve 先行（先预留再落本地状态，同事务） | owner: `internal/service/service_admission.go`、`internal/service/model.go` | 依赖： 1.2；Lane A | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'ServiceAdmission' -count=1`；预期： 预留成功才落本地 operation，缺 entitlement/未配置 reporter/远端拒绝分别以 `service_admission_required`/远端拒绝码/`identity_unavailable` 拒绝且不落本地操作；失败复查： 检查事务边界与 fail-closed 顺序。
- [x] 2.2 终态上报（delivered/实际用量、failed/0、台账转 reported）与重启恢复对账（本地 completed→delivered、记录缺失→unknown_accept、本地 failed 保持持有） | owner: `internal/service/service_admission.go`、`internal/service/lifecycle.go` | 依赖： 2.1；Lane A | 验证： 同上命令加重启用例；预期： 重启后未上报预留按本地终态对账，failed 不被误上报；失败复查： 用 service 重启测试复现，核对台账状态机。
- [x] 2.3 配置项 `service_identity_issuer`、`service_identity_endpoint`、`service_access_reporter_credential_file`（默认关闭），凭据不进入日志/回执/MCP 输出 | owner: `internal/config`、`cmd/drivebridge-server/commands.go` | 依赖： 无；Lane B（与 2.1 并行） | 验证： `python3 scripts/check.py -- go test ./... -run 'Config' -count=1`；预期： 未配置时托管写入 fail-closed，日志中无凭据明文；失败复查： grep 测试输出与日志样例确认脱敏。

## 3. 实施与验证

- [x] 3.1 SDK `NewAigora`： 长期 Key 只送 Aigora、线程安全缓存短期凭证、失败不回退直连、HTTP 客户端不跟随重定向 | owner: `sdk` | 依赖： 无；Lane B | 验证： `python3 scripts/check.py -- go test ./sdk -count=1` 及 `go test -race ./sdk -count=1`；预期： 交换失败时不向 DriveBridge 发送长期 Key；失败复查： race 失败先定位缓存锁，再核失败路径无回退。
- [x] 3.2 降级读取边界： 凭证未过期且带 `drivebridge:read` scope 时 GET/HEAD 元数据读取可用，中心不可用时写入与提权仍在线核验被拒 | owner: `internal/service/service_identity.go`、`internal/service/http.go` | 依赖： 1.1、3.1；Lane A | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： 中心不可用场景读取降级可用、托管写入被拒；失败复查： 区分凭证过期与中心不可用两类用例。

## 4. 实施与验证

- [x] 4.1 部署侧命令 `governance identity-link` 维护映射与切换记录，输出符合 ai-native-cli-output-contract，凭据只显示来源/路径/脱敏摘要 | owner: `cmd/drivebridge-server/governance.go`、`cmd/drivebridge-server/output.go` | 依赖： 1.2；Lane C | 验证： `python3 scripts/check.py -- go test ./cmd/drivebridge-server -count=1`；预期： 真实 CLI 建映射/切换/拒绝旧凭据全链路通过；失败复查： 先单跑失败用例，再核 JSON/exit code 合同。
- [x] 4.2 MCP 适配器 `aigora_url` 支持（mcp/drivebridge owner，Server 侧提供能力发现与错误码合同），客户端无需安装 CLI | owner: `internal/service/mcp.go`、`internal/service/discovery.go`（适配器实现归 mcp/drivebridge，单独交接） | 依赖： 3.1、4.1；Lane C | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'MCP|Discovery' -count=1`；预期： 能力发现报告统一身份状态，未配置时明确阻塞原因而非假装可用；失败复查： 对照 discovery 输出 schema。
- [x] 4.3 Web/通知面接入统一登录提示与错误码展示（前端归 client/drivebridge-client，本侧仅提供合同与回归） | owner: `internal/service/http.go`、`internal/service/error_messages.go` | 依赖： 4.1；Lane C | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： `unified_login_required` 等六个新错误码在 HTTP 面形状稳定；失败复查： 对照 docs/contract-map.md 错误码表。

## 5. 实施与验证

- [x] 5.1 权限、故障、迁移与端到端回归 | owner: `internal/service`、`sdk` | 依赖： 1.x、2.x、3.x；Lane V | 验证： `python3 scripts/check.py -- go test -race ./internal/service ./sdk -count=1`；预期： 全绿且包含新用例（拒绝 no tests to run）；失败复查： 将失败归因 introduced/pre-existing/environmental 后回到对应 lane。
- [x] 5.2 收尾验证与证据归档： 运行 `python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server`、`openspec validate drivebridge-aigora-service-access-v1 --strict --no-interactive`，证据写入 `temp/integration-test-runs/<run-id>/` 六件套并脱敏；同步 docs（contract-map/tenant-governance 相关章节）；真实 Identity Platform/Aigora 账号联调需单独授权并单独报告，不在本 closeout 范围；全部通过后 `openspec archive` | owner: 全项目 | 依赖： 4.x、5.1；Lane D | 失败复查： 任一命令失败先修源因再重跑整条，不以跳过用例清绿。

验证证据：`temp/integration-test-runs/20260917T142813Z-8cbc6457/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
