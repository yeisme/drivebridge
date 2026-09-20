## 1. 记录与围栏

- [x] 1.1 依赖无；串行 A。GORM `AdoptRecord` + `AdoptStorage` 幂等/mismatch + `OwnerProtectedRel`；AutoMigrate。验收：bridge 单测。 | evidence: `go test ./internal/bridge -count=1 -run 'Adopt|Storage|owner_protected|Protected'` exit 0
- [x] 1.2 依赖 1.1；串行 A。已 adopt 空间的 upload/mkdir/trash/move/copy 围栏返回 `owner_protected`；list/stat 与 CAS `objects/sha256` 例外。验收：bridge 单测。 | evidence: TestAdoptLocalZeroCopyAndFence / TestOwnerProtectedRel

## 2. CLI 与错误码

- [x] 2.1 依赖 1.1；串行 A。`storage adopt`（local/s3）复用 alias/probe/AddSpace；`storage adopt status`；信封 `copied=false`。拒绝非 local/s3。不删除 `storage add`。 | evidence: `go test ./cmd/drivebridge -count=1 -run 'TestCLI/storage'` exit 0
- [x] 2.2 依赖 2.1；串行 A。注册 `owner_protected`、`adopt_mismatch`；同步 `docs/error-codes.md`。recipe 增加 adopt 卡；doctor 对 s3 adopt 声明 `opaque_encrypted` 未自动探测。 | evidence: TestErrorCodesDocInSync；storage.txt doctor 断言

## 3. 校验

- [x] 3.1 依赖 1.2、2.2；串行 V。聚焦测试与 `openspec validate drivebridge-owner-storage-adopt-v1 --strict --no-interactive`；`CGO_ENABLED=0 go build`。 | evidence: strict valid；`CGO_ENABLED=0 go build -o /tmp/drivebridge-adopt-build ./cmd/drivebridge` exit 0
