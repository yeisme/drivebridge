## 1. 本机复制

- [x] 1.1 DuplicateFile / DuplicateDirectory / RelocateDirectory / TrashDirectory + action 路由
- [x] 1.2 测试：文件复制、目录复制、移入自身拒绝
- [x] 1.3 LocalWorkspace adapter 与文档

验证证据：`temp/integration-test-runs/20260919T132126Z-94a153d6/summary.json`（相对 `mcp/drivebridge`）。

验证：`go test ./internal/bridge -count=1 -run 'TestDuplicate|TestRelocate|TestTrashDirectory'`
