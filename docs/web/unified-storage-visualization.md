# 统一存储可视化：分工、矩阵与阶段

2026-09-25 状态校正：下方“早期落地”保留原阶段记录；当前本机受控写入以 [本机文件体验](../local-file-experience.md) 为准。新增总览、详情和云端管理见 [文件平台演进](file-platform-evolution.md)，属于待实施规格。

DriveBridge 有两个使用面：**人用 client 可视化**（本机 `drivebridge view` + 服务端网页），**Agent 用 CLI/MCP**（`cli/drivebridge` 拥有）。本设计把两类面统一到同一套存储呈现语言：任何存储类型（OneDrive、对象存储、NAS/网盘协议、本地目录）都按后端身份 + 能力矩阵如实呈现。

## 面与入口

| 面 | 入口 | 使用者 | 写能力 |
|---|---|---|---|
| 本机视图 | `drivebridge view`（loopback + 一次性启动链接） | 用户本人 | 仅经已启用的私有 owner socket 执行既有授权动作；owner 缺席时只读 |
| 服务端网页 | `drivebridge-server serve` + 本项目 `dist` | 登录用户 | 上传/版本/回收站（既有合同） |
| CLI / MCP | `drivebridge` 命令与 stdio MCP | Agent/终端 | `--enabled` 门控（既有合同） |

rclone 通用后端仍由本机模式承担；本机 local/S3/OneDrive/Google Drive 的一等入口以 [接入 recipe](../storage-recipes.md) 为准。服务端当前连接仍为平台 S3 / 自有 S3 / OneDrive；[三类云连接器规格](../../openspec/changes/drivebridge-cloud-connectors-v2/design.md) 新增 Google Drive typed adapter 的设计，不在多用户进程切换全局 rclone 配置，尚未实现或验收。

## 能力矩阵（与 `backend_capabilities` 对齐）

| 后端 | 传输 | 临时直链 | 参考交接 |
|---|---|---|---|
| OneDrive | ✓ | ✓ | ✓ |
| generic rclone（s3/drive/dropbox/box/pcloud/webdav/smb/sftp/local/alias） | ✓ | ✗ | ✗ |
| 未识别 kind | 能力未知，按只读/既有授权呈现，不猜测 | | |

版本语义同样分层：OneDrive 引用绑定 item 版本；generic 空间版本钉在「大小+修改时间」，远端变化表现为 `file_version_changed`，不静默复用旧引用。可视化不隐藏这一差异。

## 早期落地记录（client + mcp view）

以下记录当时的只读 insights 浏览切片。当前完整本机文件页已增加真实目录、固定引用内容读取与私有 socket 授权动作，不能继续用早期目录限制描述所有页面；统计与索引 fallback 仍保持只读观测语义。

- mcp view `/spaces` 每空间增补归一 `backend`（`Space.Kind()`：`""`/`onedrive`→`onedrive`，其余为具体 kind）。
- client `src/storage.ts` 单一分类源；本机 view 渲染「存储后端」徽标条（仅当 backend 字段存在）；空间下拉附后端标签。
- 服务端连接页按 kind 如实描述；未知 kind 显示「未识别的存储类型」，不冒充 S3。
- **本机只读存储浏览（两级数据源）**：insights 入口新增「存储浏览」tab。owner 运行（`mcp --enabled --insights`）时经 insights socket 实时列目录（`POST /api/v1/local/list` → socket `POST /list`，60s 超时、500 条截断 partial）；owner 缺席/未启用/失败时按错误码降级已观察索引（`GET /usage/files` 带 `dir` 过滤）并显示原因。
- **目录发现约束（实现期发现）**：两个 Provider 的 `List` 都只返回文件对象（`remember` 因 Size≤0 拒绝目录行），子目录聚合只能来自索引 `Dir` 字段——未观察过的目录不出现在导航中；面包屑 +「前往目录」手动输入补充，`..` 等逃逸段客户端与服务端双重拒绝。
- 服务端文件页：空间选择器旁后端徽标（connection kind → `backendOf`，未绑定不渲染）；文件详情显示「所属存储：label · 能力」。

## 服务端 generic rclone 连接评估（结论：暂不实现）

本节保留对“任意 rclone 后端服务端接入”的评估；它不阻止已确认的 OneDrive/S3/Google Drive typed adapter 设计，也不把其他后端晋升为一等网盘。

需求：用户在服务端网页直接连接任意 rclone 远端（Google Drive/Dropbox/WebDAV/NAS…）。结论是**另立 change、暂缓实现**，理由与前置条件：

1. **多用户 rclone 凭据隔离未决**：rclone 的全局配置与 `RCLONE_CONFIG_*` 环境是进程级的；服务端是多用户进程，README 已明确「没有在多用户进程里切换 rclone 的全局配置」。可行方向是每连接独立 rclone config 子目录 + 每用户子进程沙箱，但这是新的运行面（进程生命周期、资源上限、崩溃恢复都要定义），不是一次增量。
2. **能力边界必须先钉死**：generic 后端无直链/参考交接（`backend_capabilities`），服务端引入后受管副本语义（版本钉、S3 化、配额）要与现有 S3/OneDrive 连接对齐，导入/回存流程需要新的投影层。
3. **替代路径已可用**：generic 后端今天就能通过本机面使用（`space-add --backend generic` + CLI/MCP + `drivebridge view` 浏览）；单人多设备的个人场景下本机面已覆盖。

若未来实现：新 change 需先定义「rclone 连接器进程模型」设计（每用户/每连接的 config 隔离与生命周期），并保持 `backend_capabilities` 诚实矩阵不放宽。

## 早期后续项与替代关系

1. **文件页后端徽标深化**：文件列表行内徽标与复制引用提示按后端区分消费方式（例如 OneDrive 引用可交接、generic 仅传输）。
2. **view stat 校验入口**：该早期缺口应按当前本机文件页及 owner 合同复核，不再整体标为未实现；新信息展示由 `drivebridge-information-workspace-v2` 承接。
3. **服务端云连接**：三类 typed 后端归新规格；任意 generic 服务端连接保持上方独立评估。

## 边界

- insights 查询与索引 fallback 只读；完整本机文件页的修改经已启用私有 owner socket 执行。view 不开放远程可写监听，也不持有第二套业务数据库；实时列表仍是**已观察索引**，不能宣称全盘完整。
- 能力标签只做解释，不升级能力；generic 空间不得显示为支持直链或交接。
- 未识别后端是诚实状态，不是错误；不因未知 kind 阻断浏览。
