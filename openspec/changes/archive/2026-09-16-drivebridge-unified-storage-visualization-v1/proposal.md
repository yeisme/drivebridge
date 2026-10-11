## Why

用户需要一眼看清「哪个空间是什么存储、能做什么」：本机 view 与服务端网页面对 OneDrive、对象存储、NAS 协议等混合后端，但界面没有后端身份与能力差异的呈现，未识别类型还会被静默当作 S3 展示。同时明确产品分工：人用 client 可视化（本机 `drivebridge view` + 服务端网页），Agent 用 CLI/MCP。

## What Changes

- 新增共享存储后端分类模块（`src/storage.ts`）：能力矩阵与 mcp/drivebridge `backend_capabilities` 对齐（OneDrive=传输+临时直链+参考交接，generic=仅传输），未知 kind 按「未识别后端」诚实显示，不猜测。
- 本机 view（insights 入口）：`/spaces` 携带 `backend` 时渲染「存储后端」徽标条（空间名 + 后端 + 能力），空间筛选下拉附带后端标签；服务端 `/spaces` 未携带 `backend` 前保持静默不猜。
- 服务端网页连接页：按连接 kind 如实描述（平台 S3 / 自有 S3 / OneDrive），未知 kind 显示「未识别的存储类型」，不再静默归为 S3。
- 配套 mcp owner change：`drivebridge-view-storage-overview-v1`（view `/spaces` 增补归一 backend 投影）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-web`：ADDED「统一存储可视化」需求（后端身份如实呈现 + 能力标签不越界 + 未知类型诚实降级）。

## Impact

只改 client 呈现层；不改 API 合同、不新增写能力、不新增连接类型。通用 rclone 后端在服务端网页的连接类型扩展另行评估（多用户 rclone 凭据模型未决），本 change 不预设。

## 本机只读存储浏览（两级数据源，2.1）

- **Tier 2 实时**：owner 以 `mcp --enabled --insights` 运行时，`POST /api/v1/local/list` 经 insights socket 由持锁 owner 实时列目录（`coverage=live_listing`），列表写入已观察索引。
- **Tier 1 索引**：owner 缺席（`insights_owner_unavailable`）、未启用（`listing_disabled`）或列表失败时，降级 `GET /usage/files`（`dir` 过滤）并显示对应原因与「已观察索引，非全盘」。
- **目录发现约束**：两个 Provider 的 `List` 均只返回文件对象，子目录聚合只能来自索引 `Dir` 字段——未观察过的目录不出现，面包屑+「前往目录」手动输入补充，不做虚假树。
