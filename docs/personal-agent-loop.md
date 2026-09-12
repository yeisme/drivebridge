# 个人 Agent 文件闭环

DriveBridge 让已连接的 Agent 在允许目录内取用文件，把报告、代码产物和创作素材回存到你自己的存储。本轮为本地候选：下面的能力有离线验证，真实网盘账号、区域、客户端组合和公网服务仍需分别验收。

## 安装与首次连接

操作者可使用本地构建的独立二进制，无需另装 Go 或 rclone；视频参考校验另需 ffprobe。分发候选由仓库命令生成，尚未发布：

```bash
python3 scripts/build.py --target linux/amd64
```

输出为 `temp/distribution/` 的压缩包、SHA256SUMS 与 CLI 生成的清单。解压后把 drivebridge 放入 PATH。支持构建 linux/amd64、linux/arm64、darwin/amd64、darwin/arm64；交叉编译通过不等于目标机器实机验收。

先在本人终端完成 OAuth；已有 rclone 配置可以复用，不建立第二份 OAuth token。首轮沿用 rclone 认证，不声称设备码免配置已实现。

```bash
drivebridge auth
drivebridge config init
drivebridge config set state /absolute/private/drivebridge-state
drivebridge config set rclone_config /absolute/private/rclone.conf
drivebridge setup --remote mydrive --root-path Projects/MyProject --local-root /absolute/local/project
```

将示例 remote 名称、目录与绝对路径替换成自己的值。`config init` 拒绝覆盖已有配置；已有配置直接使用 `config set`。命令只允许支持的非秘密配置键，不能借此开启写入口。

`setup` 默认空间 ID 为 personal、单文件上限为 1 GiB；可用 `--space`、`--max-bytes` 调整。OneDrive 自动从已有 rclone 身份发现 drive 和目录 ID，不用 Graph Explorer 抄写。目录必须已经存在。多账号时指定 remote；交互终端可输入 remote 名和目录路径，管道调用缺少选择时直接报错。此命令没有默认复制整个网盘。

通用后端同样使用已有 rclone 配置：

```bash
drivebridge setup --remote nas --remote-path Projects --space nas --local-root /absolute/local/project
```

显式 `--remote-path ''` 才表示通用后端根目录。setup 只报告配置完成；它不会把“目录发现成功”冒充“文件往返成功”。

## 一次完整任务

先把要处理的输入通过现有 upload/download 交给已连接 Agent；处理结束后用同一上传接口归档。文档理解、生成任务、费用和审核由调用方负责。

```bash
drivebridge mkdir --space personal --path outputs
drivebridge upload --space personal --file report.md --dir outputs --caller report-agent --task-ref task:report-001 --json
drivebridge recent --space personal
drivebridge stat --ref 'drivebridge://file/<returned-id>' --json
drivebridge download --ref 'drivebridge://file/<returned-id>' --out report-copy.md
```

引用使用上传返回的真实值。`caller` 和 `task_ref` 是调用方声明的来源标签，不是认证凭证或审核结果；只接受短标识符，不接受正文、URL、路径或秘密。MCP upload 可使用相同的可选字段。现有 Scaena `save_output` 不变，仍仅处理其批准的产物。

相同幂等键、内容、来源和目标目录返回原操作。改变内容、标签或目标必须使用新的 `--idempotency-key`；已有文件不覆盖。CLI 同步等待上传终态，完成后返回摘要、引用与回执。远程上传若只有写权限，返回成功操作回执，不为读取结果隐式扩大授权。

本机引用绑定远端版本；远端覆盖后旧引用会被拒绝，并不自动保留历史字节。远程受管空间的固定版本引用可读取仍保留的历史版本。要保证长期可重用，必须明确保留受管副本。

## 检索、最近活动和恢复

```bash
drivebridge find --space personal --query report --path outputs --mime-type text/markdown --after 2026-09-01T00:00:00Z
drivebridge recent --space personal --pending
drivebridge operation --id '<operation-id>' --json
drivebridge resume '<operation-id>'
drivebridge cancel '<operation-id>'
```

本机 find 检索已观察索引，after 按首次索引观察时间过滤；远程检索由服务端按文件更新时间过滤。MIME 是精确匹配。索引未命中不代表网盘无文件，消费命中前必须 stat。changes 仍是现有尽力记录的增量入口，本轮未用它驱动自动归档，也不承诺无丢失订阅。

身份过期先重新认证；源文件改变后不要拿新内容恢复旧操作；目标冲突选另一个路径；配额不足先释放容量；网络中断查原操作再恢复。远程缺少上传字节时，用同一源文件和同一幂等键再次 upload，SDK 补齐缺片。恢复不重新生成素材、不触发付费任务。

## 接入客户端与远程空间

```bash
drivebridge install --client claude-code
drivebridge install --client codex
drivebridge install --client generic --json
```

注册携带解析后的绝对路径，避免客户端从不同目录启动时丢失配置。既有 Codex 注册若不匹配会拒绝修改，操作者核对原条目后再处理。本机 `--read-only` 注册不启用文件操作或交接 socket；远程只读必须在服务端签发只读 token，不能用本机开关假装收紧远程授权。

本机 state 仍由一个进程独占；CLI 与持续运行的 stdio MCP 不能同时占用同一 state。多个客户端的并发共享应使用已有 Server，或独立空间实例；本轮没有新建 daemon。终端操作前停止持有该 state 的 MCP 客户端进程。

服务端已部署时，将网页签发的限定 token 保存到用户私有文件，再配置本机适配器：

```bash
drivebridge config set remote_url https://files.example.com
drivebridge config set remote_token_file /absolute/private/agent-token
drivebridge config set local_root /absolute/local/project
drivebridge space list --json
drivebridge upload --space '<remote-space-id>' --file report.md --dir outputs --caller report-agent --task-ref task:report-001 --json
drivebridge download --ref '<returned-remote-reference>' --out report-copy.md
```

普通 upload/download/ls/stat/find/operation/resume/cancel/trash/restore 按 remote_url 路由。远程 find 查询服务端，远程命令不打开本机状态库。远程 mkdir、changes 和永久删除尚无等价合同，明确返回 remote_unsupported。两种引用格式继续保留，不自动迁移身份或存量数据。

纯远程 MCP 不读取用户本机路径：字节通过 HTTP 上传分片/下载接口传输；可在网页先上传，再交给远程 Agent。安装本机适配器后才可通过 upload/download 访问允许的本机目录。generic 后端不能伪装为支持 Provider 直链；加密空间的参考交接限制保持原样。

## 场景矩阵与证据

| 场景 ID | 用户与任务 | 产物 | 检查 | 交接出口 | 本轮状态 |
|---|---|---|---|---|---|
| report | 研究者：资料处理后归档 | Markdown 报告/附件 | 摘要、来源、固定引用、回收站恢复 | 允许目录下载/稳定引用 | first-support（离线） |
| build | 开发者：保存构建或数据处理结果 | 压缩包/文件 | 嵌套目录、幂等、摘要、越界拒绝 | 下载到另一工作目录 | first-support（离线） |
| creative | 创作者：归档素材 | 图片/视频文件 | 普通文件回存与引用校验；生成交接另验 | 文件引用/既有 Scaena 合同 | first-support（文件闭环） |
| remote-continuity | 跨设备用户：另一 Agent 继续用结果 | 远程文件版本 | 独立身份、限定 token、旧版本解析 | Server API/MCP/Web | first-support（fixture） |
| sync-retention | 多设备整理/暂存清理 | 同步与保留计划 | 冲突、删除阈值、明确清理授权 | 既有后续 OpenSpec | exploratory |

前三类共同运行：

```bash
python3 scripts/check.py -- python3 scripts/personal_loop.py
python3 scripts/check.py -- python3 scripts/remote_system.py
```

证据均位于本 owner 的 `temp/integration-test-runs/<run-id>/`，包含 runner 生成的 summary、command、stdout、stderr、env 与 artifacts。离线通过不能晋升为真实账号成熟支持。
