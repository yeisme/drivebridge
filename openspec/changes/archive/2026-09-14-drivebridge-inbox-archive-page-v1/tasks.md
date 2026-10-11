# Tasks

- [x] 1.1 产物收件箱页：导航项、4s 轮询 `/inbox`、完成/失败卡片（来源、任务 ref、错误码中文文案、恢复跳转）
- [x] 1.2 版本面板归档表单：目标空间/路径提交 `POST /archive`，复用固定版本选择流；`invalid_archive`/`unsupported` 文案
- [x] 1.3 验证：`pnpm test`（vitest 17 通过）、`tsc --noEmit`、`vite build` 全绿

验证证据：`temp/integration-test-runs/20260914T153357Z-75086f36/summary.json`（相对项目根目录）。
