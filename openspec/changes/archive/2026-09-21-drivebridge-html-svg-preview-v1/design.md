# HTML/SVG 预览设计

## 安全模型（HTML 渲染）

上传的 HTML 是不可信内容，绝不能在应用源内执行。双重防线：

1. 服务端：media 端点内联响应已带 `Content-Security-Policy: sandbox; default-src 'none'; style-src 'unsafe-inline'`（backend change 增补 `img-src data:; font-src data:`），脚本与外部网络在响应层被禁。
2. 客户端：iframe 额外加空 `sandbox` 属性（不设 allow-* token），即使响应头丢失也不继承应用源、不执行脚本、不提交表单。

登录态风险：iframe 导航 media URL 会携带会话 cookie，但沙箱文档运行在独立 opaque origin，脚本禁用后无任何读取/外传通道；`referrerPolicy="no-referrer"` 阻断引用泄漏。

## SVG 适应舞台

SVG（尤其图表导出）常见 `viewBox`-only，`<img>` 的 naturalWidth 不可靠（0 或 300×150 默认），位图查看器的缩放百分比、标注画布与导出合成都会失真。因此 SVG/ICO 走独立等比舞台（CSS object-fit: contain 填满预览区），不提供标注与像素级信息；缩放交给浏览器/舞台尺寸。位图格式继续使用完整查看器。

## 源码切换

HTML 舞台自带「渲染 / 源码」切换；源码视图在组件内自行 fetch（与文本预览相同的 media URL 与 2MB 上限），不复用 MediaPreview 的 text effect，避免 kind 分支纠缠。渲染视图不设大小上限（浏览器原生处理）。
