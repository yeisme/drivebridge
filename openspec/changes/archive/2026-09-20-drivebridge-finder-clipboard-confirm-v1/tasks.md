## 1. Finder 交互

- [x] 1.1 剪贴板状态、菜单/选择栏复制剪切粘贴、创建副本
  - owner: client
  - 范围：`src/finder-clipboard.ts`、`src/Finder.tsx`、`src/FinderMenu.tsx`、`src/api.ts`
- [x] 1.2 快捷键与焦点环
  - 范围：`src/finder-keys.ts`、`src/Finder.tsx`、`src/finder-browser.css`
- [x] 1.3 二次确认：回收站文案、永久删除 usage、本机分享撤销
  - 范围：`src/Finder.tsx`、`src/main.tsx`、`src/LocalSharing.tsx`
- [x] 1.4 文档与 i18n
  - 范围：`docs/finder-media-creative.md`、`src/i18n.ts`

## 2. 验证

- [x] 2.1 Vitest 剪贴板/快捷键；Playwright 复制粘贴、Delete 确认、搜索框不误触发
  - 验证：`pnpm test -- src/finder-clipboard.test.ts src/finder-keys.test.ts src/finder-actions.test.ts`
  - 验证：`pnpm test:e2e tests/finder-actions.spec.ts`

验证证据：Vitest `temp/integration-test-runs/20260919T094245Z-cc9b5d50/summary.json`；Playwright `temp/integration-test-runs/20260919T110544Z-de87796b/summary.json`（相对 `client/drivebridge-client`）。
