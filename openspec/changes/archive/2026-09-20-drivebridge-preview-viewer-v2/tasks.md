## 1. 查看器核心

- [x] 1.1 `media-viewer.ts` 纯逻辑：坐标换算、EXIF/PNG 解析、笔画模型、导出合成、文本统计
- [x] 1.2 图片查看器：滚轮缩放、拖拽平移、双击、旋转/镜像、全屏与工具栏
- [x] 1.3 视频查看器：速度/逐帧/循环/画中画/截图/全屏；音频速度
- [x] 1.4 标注层：工具、颜色、粗细、撤销/清空、导出标注图
- [x] 1.5 信息面板：提取器分组、悬浮折叠、降级

## 2. 格式与样式

- [x] 2.1 `mediaKind` 扩展：SVG/ICO、扩展名回退
- [x] 2.2 深色舞台、棋盘格、工具栏与信息面板样式；compact 与分享卡不受影响

## 3. 验证

- [x] 3.1 Vitest：media-viewer 逻辑 + mediaKind 新格式（18 文件 76 测试全绿；temp/integration-test-runs/20260919T142158Z-50093844）
- [x] 3.2 `pnpm build` + `pnpm test:e2e` 无回归（build ✓；preview-viewer 2 用例 ✓；全套件 9 处失败经 HEAD 基线 worktree 复测确认为既有失败，与本 change 无关）
- [x] 3.3 fixture server 截图核对外观；文档更新（temp/browser-artifacts/preview-viewer-image.png、preview-viewer-video.png）
