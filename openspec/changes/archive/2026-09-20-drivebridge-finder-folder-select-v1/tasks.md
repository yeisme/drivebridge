## 1. Finder

- [x] 1.1 文件夹选中、打开、剪贴板与粘贴
- [x] 1.2 文档与 i18n
- [x] 1.3 Vitest 剪贴板；Playwright 双击进入、复制文件夹

验证证据：Vitest `temp/integration-test-runs/20260919T114105Z-a1f83aee/summary.json`；Playwright finder-actions `temp/integration-test-runs/20260919T114143Z-1b109dd2/summary.json`（相对 `client/drivebridge-client`）。

验证：`pnpm test -- src/finder-clipboard.test.ts`；`pnpm test:e2e tests/finder-actions.spec.ts tests/finder.spec.ts`
