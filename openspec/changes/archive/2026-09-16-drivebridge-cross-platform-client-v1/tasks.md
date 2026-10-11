# Tasks

- [x] 1.1 设计文档 `docs/cross-platform-client.md`（平台矩阵、Windows 实机门、移动端定位、桌面壳评估）
- [x] 1.2 PWA：`public/manifest.webmanifest` + `public/icon.svg` + `index.html` manifest/icon link
- [x] 1.3 e2e 断言 manifest link 与 `/manifest.webmanifest` 可达；vitest/tsc/build 全量
- [x] 1.4 `openspec validate drivebridge-cross-platform-client-v1 --strict --no-interactive`
- [x] 1.5 配套 mcp change `drivebridge-cross-platform-distribution-v1` 落地（windows 分发 + 平台抽象）
- [x] 2.1 iOS `apple-touch-icon`（180×180 PNG 由 icon.svg 经 Chromium 渲染生成，接入 index.html 与 manifest icons）
- [ ] 2.2 Windows 实机验收（见设计文档五项门；需要真实 Windows 机器）——2026-09-16 决策：延期至出现真实 Windows 用户，届时重开 change 验收
- [ ] 2.3 macOS 实机验收记录补全（view 浏览器启动 + 一次完整上传/浏览回环）——2026-09-16 决策：随 2.2 一并延期，macOS 真实设备可用时补录
