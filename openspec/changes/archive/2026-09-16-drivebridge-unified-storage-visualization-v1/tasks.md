# Tasks

- [x] 1.1 `src/storage.ts` 统一后端分类（能力矩阵对齐 backend_capabilities；未知 kind 诚实降级）
- [x] 1.2 `src/storage.test.ts` 覆盖 OneDrive 特权、generic 仅传输、未知不猜测
- [x] 1.3 本机 view 存储后端徽标条 + 空间下拉后端标签（backend 缺失时静默）
- [x] 1.4 服务端连接页按 kind 如实描述（平台 S3 / S3 / OneDrive / 未识别）
- [x] 1.5 配套 mcp owner change `drivebridge-view-storage-overview-v1`（/spaces backend 投影）落地
- [x] 1.6 设计文档 `docs/unified-storage-visualization.md`
- [x] 1.7 `pnpm test`、`tsc --noEmit`、`openspec validate --strict`
- [x] 2.1 本机 view 只读存储浏览（两级数据源）：`StorageBrowser` 组件——owner 运行走 `POST /local/list` 实时目录，缺席/拒绝按错误码降级已观察索引并显示原因；子目录聚合来自索引 Dir 字段（Provider 不返回目录行），面包屑+手动「前往目录」补充；键盘导航从 tabs 数组派生（去 %4 硬编码）。配套 mcp change `drivebridge-local-storage-browser-v1`
- [x] 2.2 文件页空间选择器旁后端徽标（connection kind→backendOf，未绑定不渲染）+ 详情 Modal「所属存储」行（label+capabilityText）
- [x] 3.1 服务端 generic rclone 连接评估完成（多用户凭据隔离未决，实现另立 change；结论记 `docs/unified-storage-visualization.md`）

## 验证

- vitest：storage 矩阵、StorageBrowser 两级渲染/降级/路径拒绝/子目录导航、面板 5-tab 键盘回绕（17 通过）
- Playwright：local-insights 增 Tier1 降级与 Tier2 实时（spawn `mcp --enabled --insights`）+ 复制引用；workspace 增工具栏徽标与详情「所属存储」断言
