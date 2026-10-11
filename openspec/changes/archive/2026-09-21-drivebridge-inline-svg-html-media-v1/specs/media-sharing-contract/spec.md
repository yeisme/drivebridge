## ADDED Requirements

### Requirement: 内联矢量与网页媒体
media 读取端点的内联白名单 SHALL 包含 `image/svg+xml` 与图标 MIME；上传 MIME 嗅探 SHALL 将 `.svg` 扩展名的文本/XML 内容标记为 `image/svg+xml`；内联响应的 CSP SHALL 保持沙箱（禁脚本与外部网络）并允许内嵌 `data:` 图像与字体；下载响应行为 SHALL 保持不变。

#### Scenario: SVG 内联读取
- **WHEN** 已授权用户请求 `.svg` 版本媒体且未指定下载
- **THEN** 返回 200，Content-Type 为 `image/svg+xml`，响应带沙箱 CSP

#### Scenario: SVG 上传 MIME 修正
- **WHEN** 用户上传嗅探为 `text/xml; charset=utf-8` 的 `chart.svg`
- **THEN** 版本记录的 MIME 为 `image/svg+xml`

#### Scenario: HTML 内嵌图像
- **WHEN** 内联 HTML 预览引用 `data:` 图像或字体
- **THEN** 子资源渲染，脚本与外部网络请求仍被 CSP 阻止
