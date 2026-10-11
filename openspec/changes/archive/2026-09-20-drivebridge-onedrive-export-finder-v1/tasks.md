## 1. 回存

- [x] 1.1 ExportFile 抽出 + ExportDirectory HTTP/resume
- [x] 1.2 测试目录导出幂等
- [x] 1.3 Finder「复制到 OneDrive」

验证：`go test ./internal/service -run TestOneDriveImportExport`；`pnpm test:e2e tests/finder-actions.spec.ts`

证据：server `temp/integration-test-runs/20260919T134948Z-37d68930/summary.json`；client Playwright `temp/integration-test-runs/20260919T135106Z-ea66ce47/summary.json`。
