## 1. Finder 粘贴流

- [x] 1.1 复制作业进传输中心；directory_copy 显示文件进度
- [x] 1.2 目录树目标选择器
- [x] 1.3 会话撤销上一次移动
- [x] 1.4 文档与测试

验证证据：Vitest `temp/integration-test-runs/20260919T122139Z-460ee479/summary.json`；Playwright `temp/integration-test-runs/20260919T122209Z-1b596faf/summary.json`。

验证：`pnpm test -- src/finder-undo.test.ts src/finder-keys.test.ts`；`pnpm test:e2e tests/finder-actions.spec.ts`
