# DriveBridge

本机优先的 OneDrive 文件 MCP 工具，采用固定版本 rclone 与 Microsoft Graph。上传、下载和恢复由 DriveBridge 持有；Scaena 保留生成准入、费用控制、任务查询与 artifact 真源。接口为 pre-1.0 本地候选，尚未发布或部署。

## 本机启动

需要 Go 1.26；视频参考校验需要 `ffprobe`。无需安装 Eikona、Scaena 或 rclone CLI。首版支持 Linux/macOS 的 stdio 和 Unix socket；不提供 Windows/远程多租户服务。

```bash
go build -o drivebridge ./cmd/drivebridge
./drivebridge auth
```

`auth` 调用内置 rclone 的交互配置器：选择 OneDrive 并完成用户 OAuth。已有 rclone 配置可直接使用 `--rclone-config /absolute/path/rclone.conf`，无需重新登录。此命令只在用户终端操作；不要通过 MCP 读取或打印配置。DriveBridge 与 Graph 不保存第二份 token，Graph 在 rclone 完成认证/刷新后借用当前 access token。默认仅支持微软 global 云区域；其他区域明确拒绝。

配置后与激活前用只读预检确认就绪（支持 `--remote-url` 远程模式；不做真实账号探测）：

```bash
./drivebridge doctor --state <state> --rclone-config <rclone.conf>
```

首次真实使用步骤见 [docs/activation-local.md](docs/activation-local.md)。

空间由操作者显式配置。以下尖括号参数需要替换为实际账号与目录标识；`root-id` 是允许目录的真实 item ID，不能使用显示名称或 `root` 占位符。

```bash
./drivebridge space-add --space creative --remote '<rclone-remote>' --drive-id '<drive-id>' --root-id '<folder-item-id>' --local-root '/absolute/path/creative-files' --max-bytes 1073741824 --project '<scaena-project>' --actor '<scaena-actor>'
./drivebridge capabilities --json
./drivebridge mcp --enabled --handoff
```

将最后一条命令注册到本机 MCP 客户端即可。`space-add` 不修改已有空间身份，重复 ID 返回冲突。文件上限须按你的域/模型已验证限制设置；示例 1 GiB 是显式操作策略，不代表模型支持这么大的参考文件。文件工具允许普通创作文件；参考交接首版只接受解码验证通过的 PNG、JPEG 与 MP4 视频。视频参数继续由 Scaena 模型合同校验。

不加 `--enabled` 时只可发现能力和读取既有操作回执，上传、下载、恢复和取消默认关闭。`--handoff` 还需显式启用；socket 位于 state 目录的 `handoff.sock`，权限 0600。异常终止后若遗留 socket，确认旧进程已停止再由操作者清理，不会自动删除未知 socket。

## 通用 rclone 后端（Google Drive/Dropbox/Box/pCloud/S3/WebDAV/SMB/SFTP）

除 OneDrive 外，空间可以绑定任意已配置的 rclone 远端作为**仅传输面**后端。先用 `drivebridge auth`（或自有 rclone 配置 `--rclone-config`）在该远端完成登录，再以 `--backend generic` 建空间；`--remote-path` 钉住远端内的根（空=远端根）：

```bash
./drivebridge space-add --space gdrive --backend generic --remote '<rclone-remote>' --remote-path 'creative/refs' --local-root '/absolute/path/refs' --max-bytes 1073741824
```

能力边界由 `capabilities` 的 `backend_capabilities` 如实区分：generic 空间支持 list/stat/upload/download/恢复与稳定引用，**不提供**临时直链（`access_unsupported`）与 Scaena 参考交接（`handoff_not_available`）——这两项保持 OneDrive 专属。版本钉在 `大小+修改时间` 上，远端内容变化表现为版本漂移（`file_version_changed`），不静默复用旧引用。

软链接默认拒绝（`symlink_rejected`），防止越界逃逸；空间级 `--follow-symlinks` 显式开启后以内容副本物化上传，且指向 local root 之外的链接仍被 os.Root 约束拦截。纯 generic 空间的 doctor 中 ffprobe 降级为 info（参考校验用不到）。

内嵌后端清单见 `internal/bridge/backends.go`（alias/local/box/dropbox/drive/pcloud/s3/smb/sftp/webdav + OneDrive）；新增后端=补一行 import，无需其他接线。

## 配置文件与命令树（cobra/viper）

常用路径与 URL 可进配置文件，一次写好、所有命令复用：

```bash
./drivebridge config init          # 写 ~/.config/drivebridge/config.yaml（模板含注释）
$EDITOR ~/.config/drivebridge/config.yaml
./drivebridge config show          # 每键来源：flag / env / file / default
```

发现序：`--config <path>` > `DRIVEBRIDGE_CONFIG` > `./drivebridge.{yaml,yml,json,toml}` > `~/.config/drivebridge/config.*`；解析优先级：显式旗标 > `DRIVEBRIDGE_STATE` 等 env > 文件 > 默认。**未知键直接报错**（防打错键名静默失效）；可配置键只有 `state/rclone_config/scaena_url/scaena_token_file/remote_url/remote_token_file/local_root`。

安全边界：`--enabled/--handoff` 等"默认关闭的写入口开关"只能显式命令行，不进文件；任何 secret 永不入文件（token 文件路径可以）。

命令树：`auth`、`space add|list`、`doctor`、`capabilities`、`mcp`、`remote`、`config show|init`、`install`、`upload/download/ls/stat/operation/mkdir`（本地直连传输）、`completion`（bash/zsh/fish）。所有旗标名与旧版一致；配置就绪后 MCP 注册只需：

```bash
./drivebridge install --client claude-code   # 或 codex / generic
./drivebridge mcp --enabled --handoff
```

### 工作区目录与直连传输

agent 与终端用户共享同一套目录语义：`list`/`ls` 接受 `--path`（或 MCP `path` 参数）浏览子目录，`mkdir` 幂等建目录，`upload` 可指定 `--dir`（MCP `dir` 参数）落进目录——目标名保持 `<operation-id>-<name>` 不替换既有文件，路径逃逸一律 `path_not_allowed`。CLI 传输命令在用户本人终端执行（无 `--enabled` 门，该门只管 MCP 写入口）；缺省幂等键确定性派生，重试复用原操作：

```bash
./drivebridge mkdir --space creative --path refs/2026
./drivebridge upload --space creative --file ref.png --dir refs/2026
./drivebridge ls --space creative --path refs/2026
./drivebridge operation <operation-id>     # 轮询进度
```

doctor 新增 `config_file` 检查（来源与 0600 建议，info 级，永不阻塞就绪）。

## Agent 操作合同

通过 `tools/list` 或 `drivebridge://capabilities` 发现真实工具/schema。能力资源与工具注册表共用定义。`authorization_status` 和 `storage_status` 的 `not_probed` 不代表已连通。

| 工具 | 行为 |
|---|---|
| `list` | 列出允许 OneDrive 根目录下的直接文件，返回固定版本引用 |
| `stat` | 依据稳定引用检查账号、目录、文件身份与版本 |
| `upload` / `download` | 发起异步传输，返回 operation ID |
| `operation_get` | 查询字节进度与完成文件引用 |
| `resume` / `cancel` | 操作原 transfer，不创建生成任务 |
| `prepare_reference` | 检查引用与宿主可用性，不获取或返回 URL |
| `save_output` | 从已成功且已导入的 Scaena 任务取回成品并上传 OneDrive |

一个上传对应一个文件。目标名称为 `<operation-id>-<name>`，避免替换用户选定的既有文件；同幂等键与相同源指纹重放返回原操作。上传完成后重新读取并计算 SHA-256，不能将 ETag 当摘要。源文件改变会拒绝恢复。

文件引用固定 `space/drive/item/version`；名称仅展示。移动到空间外、跨盘或版本变化均拒绝消费。远端已有文件在首次领域交接前会下载到受控临时文件，解码/ffprobe 验证并缓存该版本摘要；这次验证下载不向第三方存储复制素材。之后才使用 OneDrive 直链交付 Provider。

下载到已有路径时，只有内容完全一致才视为恢复成功；不同内容保持原样。服务重启将未完成操作标记为 `interrupted`。恢复先检查已有远端文件；必要时整文件重传，不承诺跨进程分片续传。网络、认证、429、配额等错误使用固定代码，不输出原始错误正文。

## 与 Scaena 的本机闭环

在 **Scaena 服务进程** 配置 `SCAENA_DRIVEBRIDGE_SOCKET` 为上述 socket 的绝对路径、`SCAENA_DRIVEBRIDGE_ACTOR` 为授权 actor，然后通过原有配置显式启用已验证的参考生成 binding。这是新增适配器入口；未配置时旧流程不变。

Agent 把 `drivebridge://file/<id>` 填入既有 `createVideoProviderTask` 的 `reference_upload_ref`。Scaena 在原有权利、人物、模型及费用门检查后才通过 socket 获取 Graph 临时直链。Provider 请求使用 URL，普通媒体 JSON、预览、任务数据库和 MCP 回复只含引用与摘要。

已提交任务中的 URL 不能被这个适配器替换。提交结果不明时使用 Scaena 原查询/reconcile；当前实现不自动重试任何付费生成。确认素材下载失败后也由原业务授权决定是否创建新的关联尝试。

成品回存使用既有 Scaena HTTP API，在 DriveBridge 启动时配置服务地址与现有用户凭据：

```bash
./drivebridge mcp --enabled --handoff --scaena-url 'http://127.0.0.1:8080' --scaena-token-file '/absolute/path/private-scaena-token'
```

凭据文件必须是权限 0600 的普通文件，且对应空间配置中的 Scaena actor；也支持已有 `SCAENA_API_TOKEN` 环境变量。不要把真实 token 写入项目。端口使用你实际配置的 Scaena listener，不由 DriveBridge 自动启动。

Agent 在任务 `succeeded` 且 `import_status=imported` 后调用 `save_output(space_id, submission_ref, idempotency_key)` 并轮询 operation。宿主适配器只 GET 原任务、申请 artifact 读取 grant、从同一个 owner 下载、校验摘要并上传 OneDrive；不读取 grant 返回的任意 URL，不执行生成或 reconcile。临时 grant 不进入 MCP 输出。回存失败恢复原 operation；成品保持 Scaena 原有待审/不可导出状态。回存是用户私有副本，不批准发布。

本机已下载成品默认保留在允许目录的 `.drivebridge-output/`，方便恢复；当前不自动删除。远程 Scaena 不能读取本机 socket 时，参考交接不可用，不开放新端口或悄悄改走 TOS。

## 验证与限制

```bash
python3 scripts/check.py
python3 scripts/check.py -- python3 scripts/mcp_smoke.py
python3 scripts/check.py -- go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge
openspec validate drivebridge-local-mcp-v1 --strict
openspec validate drivebridge-host-handoff-v1 --strict
```

每次证据包含 `summary.json`、`command.txt`、`stdout.log`、`stderr.log`、`env.json` 与 `artifacts/`，失败也保留证据。运行信息仅记录脱敏离线结果。

目前验证的是本地 fake OneDrive/Graph/OAuth/Scaena、真实 stdio 和 Unix socket；真实账号、已部署 Gateway、火山区域抓取和排队有效期尚未验收，不能宣称生产可用。Gateway 转发普通 MCP 工具时无需携带 URL；参考生成还要求 Scaena 的本机 socket 可达，否则明确阻塞。

微软临时下载 URL 可能数分钟内失效，不能配置或承诺覆盖任意队列。真实账号/模型/渠道/区域组合经过代表性排队验证后才能默认启用。失败不自动新增公共分享、TOS 或媒体代理。

回滚：停止新进程或移除启用标志及 Scaena socket 配置，保留 OneDrive 文件、任务记录和 DriveBridge state。远程 Git 注册、发布、部署与真实账号操作独立执行。

## 远程个人空间模式（增量）

本机模式保持原样。新增远程模式使用 DriveBridge Server 的限定 Agent token，不读取本机 rclone 配置或把旧本机凭据自动升级为远程权限。

先在网页创建 Agent 授权，将一次显示的 token 保存到用户私有、权限 0600 的普通文件。不要把它放进聊天、日志或仓库。

```bash
go build -o drivebridge ./cmd/drivebridge
./drivebridge mcp --remote-url https://files.example.com --remote-token-file /absolute/private/agent-token --local-root /absolute/allowed/files
./drivebridge capabilities --remote-url https://files.example.com --remote-token-file /absolute/private/agent-token --local-root /absolute/allowed/files --json
```

MCP `upload` 参数为 `space_id`、相对 `local_path`、远程相对 `path`、稳定 `idempotency_key`；更新版本另加 `file_id` 和 `expected_version`。`download` 接受 `file_id`、相对 `local_path` 和可选 `version_id`。本机路径只能在固定 local root 内，下载以临时文件校验后发布，不覆盖不同内容。

`operation_get` 查询原操作。若仍缺字节，使用同一文件、同一幂等键再次调用 upload，SDK 只发送缺片；resume 不会猜本机路径，也不会发起生成。下载中断可重新调用，未验证完整的文件不会成为最终目标。

CLI 也提供 `remote --action <tool-name>`，从 stdin 读取 JSON 参数，并支持原有 `--json`、`--agent`、`--events` 和 `--explain` 输出。不要把 token 当成 JSON 参数输入。

SDK 依赖 `github.com/yeisme/drivebridge-server/sdk`。本地候选通过 go.mod 的相邻 owner replace 验证，正式独立发布前需替换为已发布版本；当前未发布。

完整离线闭环验证：

```bash
python3 scripts/check.py -- python3 scripts/remote_system.py
```

该验证启动真实服务/OIDC fixture 和本机 stdio MCP，覆盖授权、上传下载、重放、目录隔离和撤销；不会访问真实存储或产生 Provider 费用。
