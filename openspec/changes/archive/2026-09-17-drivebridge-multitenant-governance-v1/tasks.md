# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。任务分组：A=租户隔离与空间权限，B=团队治理/授权/通知，C=应急与人工恢复，D=HTTP/SDK/MCP 合同与全量验证，E=收尾。Lane 说明：A 先行（模型与迁移）；B、C 在 A 的租户/角色模型稳定后可并行；D 依赖 A/B/C；E 最后。不部署、不接真实账号、不新增组织计费平台；跨用户读取与应急均要求持久审计；Web 验证使用既有 Vitest 与 Playwright，归 client owner 单独报告。

## 1. 多租户与独立空间权限（Requirement: Tenant isolation and scoped administration / Deployment tenant boundary / Admin session inheritance）

- [x] 1.1 依赖无；串行 A。GORM 增量迁移： 租户、成员关系、团队空间表与上下文字段（先加表和字段，不动旧数据）；`Principal.UserID` 保持真实身份，成员存储 owner 与登录身份分离，既有成员默认 owner 不变；团队空间每空间独立数据 owner；角色与成员状态每请求重查；未启用治理的部署保持实例即租户与旧 admin 会话权限投影 | owner: `internal/service/tenant.go`、`internal/service/teams_model.go`、`internal/service/governance_model.go`、`internal/service/model.go` | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： 迁移幂等，旧部署不激活即行为不变，实例身份继续表示引用所属部署；失败复查： 用既有 fixture 库跑迁移并回归旧登录。
- [x] 1.2 依赖 1.1；串行 A。显式治理引导命令： 部署操作者指定首批最高管理员并启用治理，既有用户/空间归入默认租户；无自动把旧 admin 升级；激活后跨租户成员关系与资源归属逐请求检查；成员离组/租户停用后后续访问与未执行许可被拒、既有数据保留 | owner: `internal/service/governance_enrollment.go`、`cmd/drivebridge-server/governance.go`、`internal/service/tenant.go` | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'Tenant|Governance' -count=1`；预期： 隔离矩阵（多租户/多角色/猜测资源/旁路接口）全绿，旧引用与文件 ID 保留；失败复查： 先核上下文来源（服务端准入，非客户端选择），再核成员状态重查。
- [x] 1.3 依赖 1.1；串行 A。管理员只读监督： 平台/租户/team 管理员只读各自范围，空间负责人独立审批；管理员选择权限范围内空间只读时保存访问审计与站内通知，写入仍需独立权限或有效授权；独立协作编辑权限不得自批或自授 | owner: `internal/service/governance.go`、`internal/service/governance_stream.go`、`internal/service/teams_read.go` | 验证： 同上命令覆盖管理读取与越权写入；预期： 读取有审计与通知，写入无授权即拒；失败复查： 核对审计/通知保存失败时操作不得视为成功。

## 2. 团队治理、授权与通知（Requirement: Explicit approval and emergency access）

- [x] 2.1 依赖 1.x；Lane B。申请与授权有效期： 申请 24h；范围授权默认 1h、最长 8h；精确操作清单批准后 1h 一次执行，版本改变重新申请；批准/执行/撤销分离；审计及通知必须成功保存 | owner: `internal/service/governance_requests.go`、`internal/service/governance.go` | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'Governance' -count=1`；预期： 获批清单对应版本或授权条件变化时执行失败且不能修改新版本；失败复查： 重放/撤销/过期三类用例分别复跑。
- [x] 2.2 依赖 2.1；Lane B。审批与撤销竞争、站内通知与访问审计查询： 30min 汇总查看、逐任务记录下载；不存正文或凭据；停权保留数据并撤销未完成授权 | owner: `internal/service/governance_stream.go`、`internal/service/insights.go`、`internal/service/governance.go` | 验证： `python3 scripts/check.py -- go test -race ./internal/service -run 'Governance|Teams' -count=1`；预期： 并发审批/撤销无一致性破坏，审计记录无凭据/正文；失败复查： race 报告定位到事务边界后修复，不放宽隔离级别断言。

## 3. 应急与人工恢复（Requirement: Recovery cannot impersonate the owner / Key rotation / User management HTTP surface）

- [x] 3.1 依赖 1.x；Lane C。15 分钟应急： 仅平台最高管理员复核身份后启动；全程持久审计与通知 | owner: `internal/service/governance_recovery.go`、`internal/service/governance.go` | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'Recovery|Emergency' -count=1`；预期： 非最高管理员或无复核即拒，应急窗口过期自动失效；失败复查： 核对角色实时重查而非会话快照。
- [x] 3.2 依赖 3.1；Lane C。人工恢复： 本人先登记新凭据领取证明，管理员仅批准核验；凭据不向管理员返回；恢复使旧会话/旧密钥/未完成授权失效并暂停管理/审批角色，另一位有权管理员重新确认；最后一步最高管理员使用部署侧命令；激活后旧轮换/签发管理写接口以 `account_recovery_required` 拒绝并引导本人恢复 | owner: `internal/service/governance_recovery.go`、`internal/service/account_takeover.go`、`internal/service/users.go`、`cmd/drivebridge-server/governance.go` | 验证： 同上命令覆盖领取证明、暂停特权、旧接口拒绝；预期： 管理员任何路径都无法取得他人新密钥；失败复查： 区分未激活（旧合同不变）与已激活（拒绝并引导）两模式。

## 4. HTTP/SDK/MCP 合同与完整 owner 验证证据

- [x] 4.1 依赖 1.x–3.x；串行 D。旧 API 增加字段、SDK 增加上下文选择，旧列表响应形状保留且不返回凭据/哈希；治理启用后用户列表仅平台最高管理员可读；MCP 面权限与错误码合同同步；现有单租户未激活部署继续兼容 | owner: `internal/service/http.go`、`internal/service/users.go`、`internal/service/mcp.go`、`sdk` | 验证： `python3 scripts/check.py -- go test ./internal/service ./sdk -count=1`；预期： 旧合同回归全绿，新字段为增量可选；失败复查： 对照 docs/contract-map.md 逐端点核对。
- [x] 4.2 依赖 4.1；串行 D。完整验证矩阵： 多租户/多角色、猜测资源、旁路接口、审批重放/撤销/过期、人工恢复、迁移重启；运行 `python3 scripts/check.py -- go test -race ./internal/service ./sdk -count=1`；每个 owner 保存脱敏验证证据至 `temp/integration-test-runs/<run-id>/` 六件套 | owner: `internal/service`、`sdk` | 预期： race 全绿、证据齐全无凭据/明文密钥；失败复查： 归因 introduced/pre-existing 后回对应 lane，不以删用例清绿。

## 5. 收尾

- [x] 5.1 依赖 4.x；串行 E。运行 `python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server`、`openspec validate drivebridge-multitenant-governance-v1 --strict --no-interactive`；同步 `docs/tenant-governance.md`、`docs/contract-map.md` 与迁移/回滚说明（上线前备份 metadata、instance ID 与密钥；新租户产生写入后禁止旧版本打开数据库）；无真实部署；通过后 `openspec archive` | owner: 全项目 | 预期： 四命令全绿、strict 通过、文档与实现一致；失败复查： 修源因后整条重跑。

验证证据：`temp/integration-test-runs/20260916T111813Z-cca2be3d/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
