# 实施与验收任务

- [x] 1.1 新增 `connections` GORM 表与迁移：字段/索引/软删；`connection_id` slug 校验；与 `spaces` 同库。
- [x] 1.2 实现 `connection add/set/remove`：字段校验（http(s) base_url、space_id 存在性、credential_source 枚举，keychain 明确未实现报错）；写入只经 CLI。
- [x] 1.3 实现 `connection list/show --json` 只读投影：字段白名单（connection_id/label/base_url/instance_id/space_id/project_ref/credential_source/credential_configured/synthetic），断言无 token、token 文件路径、临时 URL 泄露；合成默认连接投影不落库。
- [x] 1.4 兼容与迁移：注册表为空时行为不变；全局 remote 配置映射为默认连接的测试覆盖。
- [x] 2.1 单测：CRUD/校验/合成投影/信封白名单；`go test ./...`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge` 绿。
- [x] 2.2 集成验证：`python3 scripts/check.py` connection 场景（add→list→set→show→remove 全流程 + 拒绝路径）；脱敏证据入 `temp/integration-test-runs/<run-id>/`。

完成证据（2026-09-18）：`internal/bridge/connection.go`（GORM 表+校验+软删）、`cmd/drivebridge/connection.go`（add/set/remove/list/show + 只读投影白名单 + 空注册表 synthetic 默认连接）；`go test ./...`、`go vet ./...`、`CGO_ENABLED=0 go build` 全绿；CLI 冒烟（含 connection_conflict/keychain 未实现拒绝、无 token/token_file 泄露）证据 temp/integration-test-runs/connection-20260918T021242Z-1047967/summary.json（stdout.log 已脱敏真实用户 remote_url）。
