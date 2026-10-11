## 1. 实施与验证

- [x] 1.1 ArchiveVersion 入口受管目标校验：not_found/unsupported 早拒绝（零字节/零预留/零操作）
- [x] 1.2 测试：非受管目标早拒绝 + 既有钉住/冲突用例回归

验证证据：`temp/integration-test-runs/20260914T155612Z-bcdd63a9/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
