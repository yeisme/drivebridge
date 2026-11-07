# consumer-finder-interactions Specification

## Purpose
定义 C 端网盘文件浏览器的双层紧凑顶栏、彩色文件类型徽标、可读类型文案、桌面鼠标框选、底部悬浮多选操作胶囊栏、全画布拖拽遮罩、右下角传输悬浮窗以及沉浸式 Lightbox 预览体验。

## ADDED Requirements

### Requirement: Compact two-tier file header and human-readable file metadata
文件浏览区 SHALL 将路径面包屑、目录搜索、视图切换与筛选收敛在紧凑顶栏内，桌面与移动端列表/网格 SHALL 使用彩色语义文件类型徽标与中文可读文件类型名，不得在常规文件列表中直接展示原始 MIME 字符串（如 `text/plain; charset=utf-8`）。

#### Scenario: File viewport starts high and avoids raw MIME strings
- **WHEN** 用户在 390px 或 1440px 视口浏览含 Markdown 与文本文件的目录
- **THEN** 文件滚动区顶边 `y ≤ 200px`，无横向溢出，且文件副标题展示可读类型/大小/时间而非原始 MIME

### Requirement: Marquee selection and floating contextual action dock
桌面端支持在文件列表/网格空白处拖拽框选多项；当选中至少一项文件或文件夹时，画布底部 SHALL 浮现多选操作胶囊栏，提供与当前空间权限匹配的批量下载、分享、移动、复制、复制引用与移入回收站动作。

#### Scenario: Selecting files reveals contextual floating bar
- **WHEN** 用户勾选或框选 2 个文件
- **THEN** 底部多选胶囊栏浮现并显示 `已选 2 项` 及可用操作，按 `Escape` 或点击取消选择立即收起

### Requirement: Collapsible bottom-right transfer dock with honest durability states
文件上传、续传或进行中传输触发时，客户端右下角 SHALL 显示可折叠传输悬浮窗，诚实区分上传中、云端保存校验中、已完成与中断待续传状态，不得在服务端耐久校验完成前显示已完成。

#### Scenario: Verifying upload is not prematurely marked complete
- **WHEN** 上传操作处于 `verifying` 阶段
- **THEN** 右下角传输悬浮窗显示校验保存中状态，待服务端返回 `completed` 后才转为已完成并允许直接预览结果

### Requirement: Immersive Lightbox preview with progressive technical disclosure
文件预览弹窗 SHALL 突出媒体与正文阅读区域，右侧详情抽屉默认展示可读文件属性与版本时间线，底层 `file_id` 与 `version_id` 哈希收敛于可展开的「高级标识与 AI 引用」区域，并保留一键复制固定版本引用动作。

#### Scenario: Copy fixed AI reference without cluttering primary preview
- **WHEN** 用户打开文件预览并展开右侧详情栏
- **THEN** 首屏优先展示可读文件信息、下载与历史版本，点击「复制文件引用」仍复制固定版本引用，展开高级区域可核对完整文件与版本标识

### Requirement: Tactile 3D dog-ear file artwork and spring-physics choreography
文件网格与列表 SHALL 使用微立体折角文档卡（`FileArtwork`）与双层彩色文件夹徽标提升类型辨识度；主导航激活态、视图切换器、多选悬浮胶囊坞与传输悬浮窗 SHALL 提供物理弹簧过渡，且在 `prefers-reduced-motion: reduce` 或应用开启「减少动效」时自动降级为零位移。

#### Scenario: Reduced motion disables spring displacement cleanly
- **WHEN** 浏览器或个人设置启用 `reduced-motion`
- **THEN** 所有导航切换、文件卡片与悬浮坞保持状态完整可见且无位移动画
