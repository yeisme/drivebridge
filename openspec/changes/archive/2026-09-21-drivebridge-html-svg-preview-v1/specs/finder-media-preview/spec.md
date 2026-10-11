## ADDED Requirements

### Requirement: HTML 沙箱渲染预览
HTML 文件（`html`/`htm`/`xhtml` 扩展名或 `text/html`、`application/xhtml+xml` MIME）SHALL 在沙箱 iframe 中渲染预览；渲染 SHALL 禁用脚本执行与外部网络资源，且 SHALL NOT 继承应用登录源；SHALL 提供「渲染 / 源码」视图切换，源码视图遵循文本预览的大小上限并显示行数与字符数。

#### Scenario: 打开 HTML 显示渲染结果
- **WHEN** 用户打开 `.html` 文件详情
- **THEN** 预览区在沙箱 iframe 中显示渲染结果，脚本不执行、外部资源不加载

#### Scenario: 切换源码
- **WHEN** 用户在 HTML 预览中切换到源码视图
- **THEN** 显示文件文本内容与行数·字符统计，超过 2MB 提示下载查看

#### Scenario: 恶意脚本不执行
- **WHEN** HTML 内嵌 `<script>` 或引用外部脚本
- **THEN** 脚本被阻止，预览其余静态内容正常渲染

### Requirement: SVG 适应舞台
SVG 与 ICO 预览 SHALL 在不依赖固有尺寸的等比适应舞台中渲染，SHALL NOT 提供位图标注导出；位图格式 SHALL 继续使用完整图片查看器。

#### Scenario: viewBox-only SVG
- **WHEN** SVG 只有 `viewBox` 没有 width/height
- **THEN** 舞台仍完整等比显示图形

#### Scenario: 位图查看器不受影响
- **WHEN** 用户打开 PNG/JPEG
- **THEN** 仍进入缩放、平移与标注查看器
