## 1. 实施与验证

- [x] 1.1 ListAgents 列名修复 + RotateUserKey 过期守卫（单测覆盖）
- [x] 1.2 HTTP rotate 端点（admin 门 + 非 admin 403 + 旧 key 即死）+ 单测；全套绿

验证证据：`temp/integration-test-runs/20260916T031410Z-b5caedda/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
