# 实施与验收任务

- [x] 1.1 依赖无。GORM：File.trashed_at、Space.trash_retention_days 缺省 30；Open 回填旧行。验收：`go test ./internal/service -run TrashRetention -count=1`。 | evidence: files_trash.go / files_trash_test.go
- [x] 1.2 依赖 1.1。Sweep + batch restore/purge HTTP；错误码入 errorMessages。验收：`go test ./internal/service -run 'TrashRetention|TrashBatch' -count=1`。 | evidence: files_trash_test.go
- [x] 1.3 依赖无。key-directory revoke/delete + can_revoke/can_delete。验收：`go test ./internal/service -run KeySelfDelete -count=1`。 | evidence: key_delete_test.go
- [x] 1.4 依赖 1.2 1.3。docs/key-management.md 与 docs/README 链接。`openspec validate drivebridge-key-delete-trash-batch-v1 --strict --no-interactive`。
- [x] 1.5 依赖 1.3。UserKey.disabled、disable/enable HTTP、认证拒绝禁用 Key、CLI users disable/enable。验收：`go test ./internal/service -run KeyDisableEnable -count=1`。 | evidence: key_disable_test.go
