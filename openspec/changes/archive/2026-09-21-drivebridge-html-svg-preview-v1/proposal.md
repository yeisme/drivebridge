## Why

HTML 与 SVG 是创作 agent 最常交付的产物（图表、原型页、信息图），用户拿到 `drivebridge://…/file/…/version/…` 引用后需要在网盘里直接看到渲染结果。当前预览把 HTML 当纯文本折叠在源码 `<pre>` 里，SVG 依赖固有尺寸进入位图查看器——只有 `viewBox` 没有 width/height 的导出图 naturalWidth 不可靠，标注画布无法合成；配合后端内联白名单缺 `image/svg+xml`（另立 backend change），真实 SVG 在部分路径直接 `preview_unsupported`。

## What Changes

- `mediaKind` 新增 `html` kind：`html`/`htm`/`xhtml` 扩展名或 `text/html`（及 `application/xhtml+xml`）MIME 路由到专用渲染舞台；`TEXT_EXTENSIONS` 不再把 `html` 当文本回退。
- HTML 渲染舞台：沙箱 iframe（客户端 `sandbox` 属性 + 服务端 `Content-Security-Policy: sandbox` 双重防线：禁脚本、禁外部网络、不继承应用源），提供「渲染 / 源码」切换；源码视图复用文本预览的 2MB 上限与行数·字符统计。
- SVG/ICO 适应舞台：`mediaKind` 识别不变（仍为 image），但渲染走不依赖固有尺寸的等比适应舞台（object-fit contain），位图查看器（缩放/标注/信息）保留给位图格式。
- 图标与缩略图徽标：html 使用网页图标与「网页」徽标。
- 后端配套（backend-server/drivebridge 另立 change `drivebridge-inline-svg-html-media-v1`）：上传嗅探将 `.svg` 修正为 `image/svg+xml`、内联白名单补 SVG/图标 MIME、内联 CSP 允许 `data:` 图像与字体。

## Capabilities

### Modified Capabilities

- `finder-media-preview`：新增 HTML 沙箱渲染预览与 SVG 适应舞台两个需求；既有查看器、标注、信息提取需求不变。
