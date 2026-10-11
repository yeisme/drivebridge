## 1. 实施与验证

- [x] 1.1 `authMode`/`keyLogin` API 封装 + 登录页双路径（mode 如实呈现；密钥不落 storage）
- [x] 1.2 用户密钥管理页：列表/批量签发/一次性明文弹窗/吊销确认；导航按 me.admin 条件渲染
- [x] 1.3 backend `cmd/drivebridge-fixture` 预置固定 e2e 密钥（普通+admin；fixtures.Start 不动）
- [x] 1.4 Playwright e2e：密钥登录（admin/普通）/签发-一次性展示-列表/吊销/普通用户无管理导航；`pnpm test`、`pnpm build`、`pnpm test:e2e` 全绿

验证证据：`temp/integration-test-runs/20260915T040135Z-7d5ef90a/summary.json`（client e2e 全量 7 passed）；backend 侧证据 `backend-server/drivebridge/temp/integration-test-runs/20260915T040214Z-efa54c2d/summary.json`（go test -race ./... 全绿）。
