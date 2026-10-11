## 1. 识别与渲染

- [x] 1.1 `mediaKind` 新增 `html`（`html`/`htm`/`xhtml` 扩展或 `text/html`、`application/xhtml+xml` MIME）；`TEXT_EXTENSIONS` 移除 `html?`
- [x] 1.2 HTML 沙箱渲染舞台：空 `sandbox` iframe、`referrerPolicy="no-referrer"`、「渲染 / 源码」切换、源码 2MB 上限与行数·字符统计（fetch 一次/URL，`loadedFor` ref 防 effect 自中止）
- [x] 1.3 SVG/ICO 适应舞台：不依赖固有尺寸等比渲染（`.vector-stage`），compact 与全尺寸一致；位图查看器与标注保留给位图
- [x] 1.4 图标（Globe）与缩略图「网页」徽标

## 2. 验证

- [x] 2.1 Vitest：mediaKind html 识别 + isVectorImageFile 用例（18 文件 79 测试全绿；temp/integration-test-runs/20260920T075324Z-9cd13cfd）
- [x] 2.2 `pnpm build` ✓ + `pnpm test:e2e` preview-viewer：新增 HTML 沙箱/源码/SVG 用例 ✓（temp/browser-artifacts/preview-viewer-html.png、preview-viewer-svg.png）；既有「图片查看器」用例在 HEAD 基线（client src 全量还原后重跑）与本 change 同败=滚轮缩放既有失败，与本 change 无关
- [x] 2.3 fixture server 实测截图证据落 temp/browser-artifacts/（HTML 渲染+源码、SVG 舞台）
