## 1. 合同与实现

- [x] 1.1 CopyDirectory / MoveDirectory / TrashDirectory + HTTP + SDK
- [x] 1.2 测试：复制树、冲突改名、移入自身拒绝、回收后目录消失、幂等 resume
- [x] 1.3 `docs/media-sharing.md` 增加目录前缀合同

验证证据：`temp/integration-test-runs/20260919T114105Z-7ea54638/summary.json`（相对 `backend-server/drivebridge`）。

验证：`go test ./internal/service -count=1 -run 'TestCopyDirectory|TestMoveDirectory|TestTrashDirectory'`
