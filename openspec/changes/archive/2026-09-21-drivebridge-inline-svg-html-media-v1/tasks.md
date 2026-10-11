## 1. 实现

- [x] 1.1 `detectMIME` helper：`.svg` 扩展 + text/xml、text/plain、application/xml 嗅探结果 → `image/svg+xml`；files.go 两条上传路径接入
- [x] 1.2 `inlineMIME` 加 `image/svg+xml`、`image/x-icon`、`image/vnd.microsoft.icon`
- [x] 1.3 media 内联 CSP 增补 `img-src data:; font-src data:`，sandbox/default-src 'none'/style-src 'unsafe-inline' 不变

## 2. 验证

- [x] 2.1 Go 单测 `TestInlineSvgHtmlMedia`：SVG 上传 MIME 修正为 image/svg+xml、内联 200 + 沙箱 CSP + inline Disposition、HTML 内联 CSP、下载 lane attachment 不变
- [x] 2.2 `go vet ./...` ✓、`CGO_ENABLED=0 go build ./cmd/drivebridge-server` ✓、`go test ./...` 13 包 6 测试包全绿
- [x] 2.3 客户端 e2e（fixture 即真实后端）覆盖 SVG/HTML 上传后内联渲染（preview-viewer 新用例 ✓）
