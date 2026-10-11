# finder-media-preview Specification

## Purpose
定义 Finder 中图片、视频、文本、HTML 和 SVG 的阅读与交互预览，包含分栏、画廊及完整阅读器之间的选择和版本一致性。预览保留沙箱隔离、真实失败状态与可访问操作入口。

## Requirements

### Requirement: 图片查看器缩放平移
非 compact 预览的图片 SHALL 支持滚轮以光标为锚点缩放、拖拽平移、双击在适应窗口与 1:1 间切换，并提供缩放、适应窗口、1:1、旋转、水平/垂直镜像与全屏控件。

#### Scenario: 光标缩放
- **WHEN** 用户在图片上滚动滚轮
- **THEN** 缩放以光标位置为锚点，光标下的图像点保持不动

#### Scenario: 双击切换
- **WHEN** 用户双击图片
- **THEN** 在适应窗口与 1:1 之间切换，并重置平移

### Requirement: 图片标注
图片查看器 SHALL 提供画笔、矩形、箭头标注工具与颜色、粗细选择，标注 SHALL 绘制在原始分辨率坐标系并随缩放、旋转保持对齐；SHALL 支持撤销、清空与导出带标注的 PNG。标注 SHALL NOT 写入服务端或持久化。

#### Scenario: 缩放后补画对齐
- **WHEN** 用户放大图片后继续绘制
- **THEN** 新笔画落在光标对应的原始像素位置，与已有笔画对齐

#### Scenario: 导出标注图
- **WHEN** 用户点击导出标注图
- **THEN** 下载按当前旋转/镜像合成原图与标注的 PNG，原文件不变

### Requirement: 预览信息提取
查看器 SHALL 提取并在可折叠信息面板展示：JPEG EXIF 关键字段（相机、拍摄时间、曝光、ISO、光圈、焦距、GPS、方向）、PNG 尺寸与位深、视频/音频分辨率与时长、文本行数与字符数。提取 SHALL 在浏览器本地完成，失败时静默降级只展示已知字段。

#### Scenario: JPEG 显示拍摄信息
- **WHEN** 用户打开含 EXIF 的 JPEG
- **THEN** 信息面板显示相机与曝光等字段

#### Scenario: 无元数据降级
- **WHEN** 文件不含可解析元数据
- **THEN** 信息面板只显示尺寸、大小、类型等基础字段，不报错

### Requirement: 视频查看器控制
非 compact 预览的视频 SHALL 在原生控制条之外提供播放速度（0.25–2×）、逐帧步进、循环、画中画（能力可用时）、截取当前帧 PNG 与全屏控件；音频 SHALL 提供播放速度。

#### Scenario: 逐帧步进
- **WHEN** 用户点击逐帧前进
- **THEN** 视频暂停并前进约 1/30 秒

#### Scenario: 截取当前帧
- **WHEN** 用户点击截取当前帧
- **THEN** 下载当前帧的 PNG

### Requirement: 预览格式识别扩展
`mediaKind` SHALL 识别 SVG 与 ICO 为图片；MIME 缺失或为通用二进制时 SHALL 按扩展名识别常见视频、音频与文本（代码、配置、Markdown、CSV）格式。

#### Scenario: 无 MIME 的 Markdown
- **WHEN** `.md` 文件 MIME 为 `application/octet-stream`
- **THEN** 按文本预览

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

### Requirement: 分栏文件名称与阅读可达

分栏视图 SHALL 显示有内边距的类型图标和文件名，当前文件列宽 280–320px，名称最多两行且可取得完整名称。单击 SHALL 选择，双击、Enter 或明确阅读动作 SHALL 打开同一固定版本的既有完整阅读器；文件夹选择与进入保持原合同。

#### Scenario: 长中文名称与扩展名

- **WHEN** 当前列含长中文名称、长 ID 名称或表格文件
- **THEN** 文字不越出列框，扩展名和类型可辨，完整名称及打开阅读器入口可达；行不会压成普通按钮高度

#### Scenario: 分栏预览空间不足

- **WHEN** Finder 内容宽度不足 880px
- **THEN** 文件列表保留可读宽度，完整阅读通过已有阅读器打开，不挤出狭窄且不可用的内嵌预览

### Requirement: 画廊保留完整阅读入口

画廊 SHALL 显示自适应预览舞台、名称与文件信息及可滚动缩略条；桌面舞台至少 320px 高，窄屏至少 240px。缩略项 SHALL 使用内容布局、两行文件名和至少 80px 图像区域，不能继承单行按钮的固定高度。打开阅读器 SHALL 保留选中版本及权限上下文。

#### Scenario: 阅读文档或不支持的文件

- **WHEN** 在画廊选择 PDF、文本、音视频、Office 或未知格式
- **THEN** 支持格式可进入原完整阅读器，转换中或不支持格式显示真实状态及允许的下载路径，不显示伪造的预览

#### Scenario: 键盘切换视图与文件

- **WHEN** 用户在画廊按方向键切换后按 Enter，或从其他视图切换到画廊
- **THEN** 选中对象与固定版本一致，Enter 打开阅读；焦点在输入框、媒体控制或弹层时不劫持按键
