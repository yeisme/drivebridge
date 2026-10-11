# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。任务分组：A=服务端接口实现，V=身份隔离与回归验收，D=收尾。Lane 说明：A 先行；V 依赖 A；D 最后。无数据库迁移，不改变旧 users 列表/轮转/撤销合同；展示归 client/drivebridge-client（Web 浏览器验证在该 owner 单独执行并单独报告）。

## 1. Key 目录与标签接口（Requirement: Scoped key directory / Identity-preserving key labels）

- [x] 1.1 依赖无；串行 A。实现 `GET /api/v1/key-directory` 与 `POST /api/v1/key-directory/{id}/label`：仅返回公开元数据（所属用户 ID、user_name、created_at、current，不含 TokenHash 或明文）；所有请求经现有认证、CSRF、context binding 与 audit wrapper；Agent 与 service identity 不可访问；普通身份和治理模式严格按 `p.UserID` 筛选，非治理管理员保留旧全局范围；数据空间 DataOwner 不作为凭据身份；`current` 从当前 Bearer Key 或浏览器 session 的 OriginKey 得出 | owner: `internal/service/key_directory.go`、`internal/service/http.go`、`internal/service/auth.go` | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'KeyDirectory' -count=1`；预期： 治理管理员无论选中何种数据空间仅见本人 Key，返回体无任何哈希/凭据字段；失败复查： 对照 users 旧合同确认筛选路径未复用数据空间授权。
- [x] 1.2 依赖 1.1；串行 A。标签修改语义： trim 后 1–80 字、拒绝换行与 NUL、归档 Key 不可修改；仅改 Name，Owner/TokenHash/权限/有效期保持不变，不允许通过重命名重绑身份；Key 始终绑定原用户，团队与空间权限仍由既有治理合同决定 | owner: `internal/service/key_directory.go`、`internal/service/error_messages.go` | 验证： 同上命令覆盖改名、空名称、归档 Key、跨用户目标；预期： 他人 Key 标签修改被拒，改名前后权限投影不变；失败复查： 区分校验失败（400 类）与越权（403 类）用例。

## 2. 身份隔离、回归与收尾

- [x] 2.1 依赖 1.x；串行 V。Go race 矩阵验收： 身份映射、无凭据泄露、跨用户拒绝、改名不变更权限、禁用空名称、Agent 拒绝、治理启用后管理员范围收敛；旧 users 合同（列表形状、轮转、撤销）回归 | owner: `internal/service`（`key_directory_test.go`、`users_test.go`） | 验证： `python3 scripts/check.py -- go test -race ./internal/service -count=1`；预期： 全绿且治理开/关两套模式均覆盖；失败复查： 归因后回 1.1/1.2，不以放宽断言清绿。
- [x] 2.2 依赖 2.1；串行 D。收尾： 运行 `python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server`、`openspec validate drivebridge-key-directory-v1 --strict --no-interactive`；同步 docs（key-management/contract-map 相关章节）与 client 侧切换说明；证据写入 `temp/integration-test-runs/<run-id>/`；Web 端通过真实标签接口的浏览器验证由 client/drivebridge-client owner 执行并单独报告；回滚路径为客户端改用旧元数据并撤销新路由，持久化标签保留且兼容旧版本 | owner: 全项目 | 预期： 四命令全绿、strict 通过；失败复查： 修源因后整条重跑。

验证证据：`temp/integration-test-runs/20260916T163021Z-7a45512c/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
