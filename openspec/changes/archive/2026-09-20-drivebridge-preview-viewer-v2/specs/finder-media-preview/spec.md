## ADDED Requirements

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
