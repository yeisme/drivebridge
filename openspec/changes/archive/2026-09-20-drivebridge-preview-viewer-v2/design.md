# Design

## 结构

- `src/media-viewer.ts`：纯逻辑，便于 Vitest 直测——
  - `displayToNatural` / `panForZoom`：显示坐标 ↔ 原始像素坐标的矩阵换算（旋转 90° 倍数 + 镜像 + 缩放 + 平移），供光标缩放与标注取点共用。
  - `parseJpegExif`：最小 EXIF 解析（APP1/TIFF IFD0 + Exif SubIFD + GPS IFD），只取展示字段，不引依赖。
  - `parsePngHeader`：IHDR 尺寸、位深、颜色类型。
  - `ViewerStroke` 与 `drawStrokes`：标注笔画模型（pen/rect/arrow，原始分辨率坐标）与画布绘制。
  - `composeExport`：原图 + 标注层按当前旋转/镜像合成导出画布。
  - `textStats`：行数/字符数统计。
- `src/Media.tsx`：`MediaPreview` 对外 API 不变（`file`/`sourceURL`/`compact`）。非 compact 时按类型渲染 `ImageViewer` / `VideoViewer`；工具栏、标注层、信息面板都内聚在查看器里，quicklook 与后续复用方零改动。

## 缩放模型

zoom 相对「适应窗口」比例（fitScale 由舞台 ResizeObserver 与图片自然尺寸算出，上限 1，小图不放大到模糊）。1:1 = zoom/fitscale 换算。旋转或镜像后重置平移并回到适应窗口；滚轮以光标为锚点缩放（`panForZoom` 求新平移）；拖拽平移；双击在适应窗口与 1:1 间切换。显示百分比 = fitScale × zoom。

## 标注

笔画记录在原始分辨率坐标系，画布作为变换包装层的子元素随缩放旋转联动，取点走 `displayToNatural`，无手工逆变换。导出按当前旋转/镜像合成（画布尺寸取旋转后包围盒），SVG 源等导致画布污染时给出行内失败提示。标注只在当前会话有效，切换文件/版本即清空；服务端持久化留给后续 change。

## 信息提取

`onMeta` 回调把查看器加载到的元数据（自然尺寸、视频分辨率/时长、文本统计、EXIF）汇入信息面板。面板悬浮于舞台右上，可折叠；条目按类型分组，未来新增提取器只加 extractor 不动查看器。EXIF 仅对 JPEG 且文件 ≤ 20MB 时拉取媒体字节解析，其余类型跳过（尺寸仍来自解码结果）。

## 无障碍与键盘

工具栏按钮全部 `aria-label` + `title`；舞台可聚焦，`+`/`-` 缩放、`0` 适应窗口、`1` 1:1、`r` 旋转；全局方向键切换文件的既有行为不变；视频原生控制条不拦截。

## 验证

- `pnpm test`（media-viewer 纯逻辑 + mediaKind 新格式）。
- `pnpm build`。
- `pnpm test:e2e`（既有套件不回归）+ fixture server 截图人工核对外观。
