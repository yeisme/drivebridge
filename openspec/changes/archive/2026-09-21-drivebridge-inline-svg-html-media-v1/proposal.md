## Why

上传 MIME 嗅探用 `http.DetectContentType`，不识别 SVG——`.svg` 文本内容被存成 `text/xml`/`text/plain`；media 端点内联白名单 `inlineMIME` 又缺 `image/svg+xml` 与图标 MIME，SVG 一旦以正规 MIME 存入即在内联读取路径返回 `preview_unsupported`。HTML 内联预览依赖响应 CSP：当前 `default-src 'none'` 禁掉一切子资源，静态图表 HTML 里的内嵌 `data:` 图像与字体无法渲染。

## What Changes

- 上传 MIME 嗅探：新增 `detectMIME` helper，`.svg` 扩展且内容嗅探为文本/XML 时修正为 `image/svg+xml`，两条上传路径（分片组装与预发布校验）共用。
- `inlineMIME` 白名单加入 `image/svg+xml`、`image/x-icon`、`image/vnd.microsoft.icon`。
- media 内联响应 CSP 增补 `img-src data:; font-src data:`（保持 `sandbox`、`default-src 'none'` 与禁脚本、禁外部网络不变）。
- 下载（`?download=true`，attachment）行为不变。

## Capabilities

### Modified Capabilities

- `media-sharing-contract`：新增「内联矢量与网页媒体」需求。
