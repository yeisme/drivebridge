# 首次真实使用 runbook（本机创作闭环）

把 DriveBridge 从本地候选推进到真实账号激活。所有 OAuth 与账号步骤在**用户本人终端**完成；Agent 不代持凭据、不读取或打印配置。真实账号验收与 fixture 验证分开记录，不用本地结果宣称生产可用。

## 0. 前置条件

- Go 1.26；`ffprobe`（MP4 参考校验必需）
- 需要参考交接/成品回存时：本机 Scaena 服务（见第 6 步）
- 仅支持微软 global 云区域与 Linux/macOS

## 1. 微软 OAuth（用户终端，一条命令）

```bash
go build -o drivebridge ./cmd/drivebridge
./drivebridge auth
```

`auth` 默认走 device-code 流：终端打印验证页地址与一次性代码，你在浏览器完成登录即可——无需注册应用、无需安装向导。凭据只保存在 rclone 配置里，DriveBridge 不保存第二份 token，也不打印任何凭据。默认 remote 名为 `onedrive`（`--remote` 可改）；已有同名凭据会被拒绝，换名或先删除旧节。旧 rclone 交互配置器保留为 `auth --rclone-interactive`；已有 rclone 配置可直接 `--rclone-config /absolute/path/rclone.conf`，无需重新登录。

## 2. drive-id 与 root-id 自动发现（原手抄步骤已折叠）

- `./drivebridge auth --auto`：登录后自动发现默认 drive 与其根目录，写入 rclone 配置（`drive_id`/`root_folder_id`）并打印。
- `./drivebridge auth --root-name '<文件夹名>'`：按根目录下文件夹**精确名称**选择允许根，同样落盘。
- 需要更深的路径时用 `./drivebridge space add --root-path 'Projects/My folder'`（按路径发现 item ID）。Graph Explorer 手查 `GET /me/drive`、`GET /me/drive/root/children` 仍是兜底手段，不再是必经路径。

## 3. 配置空间

```bash
# 首选：消费 auth --auto 发现的身份
./drivebridge space add --space creative \
  --remote onedrive --auto \
  --local-root /absolute/path/creative-files \
  --max-bytes 1073741824 \
  --project '<scaena-project>' --actor '<scaena-actor>'

# 或按根目录下文件夹名选择（同一 remote 的非等价路径）
./drivebridge space add --space creative --remote onedrive --root-name 'Projects' --local-root ... --max-bytes ...

# 或显式手抄 ID（兼容旧流程）
./drivebridge space-add --space creative \
  --remote '<rclone-remote>' --drive-id '<drive-id>' --root-id '<folder-item-id>' \
  --local-root /absolute/path/creative-files \
  --max-bytes 1073741824 \
  --project '<scaena-project>' --actor '<scaena-actor>'
```

`--auto/--root-name/--root-path/--root-id` 互斥（root_selection_conflict）。`--project/--actor` 仅成品回存需要；纯传输空间可省略。`--max-bytes` 按你的域/模型已验证上限设置。空间身份绑定后不可改绑。

## 4. 预检（新命令）

```bash
./drivebridge doctor --state ~/.config/drivebridge --rclone-config ~/.config/rclone/rclone.conf
```

按名输出 `pass/fail/info`，ready 以 0 退出；支持 `--json/--agent/--events/--explain`。doctor 只读：不创建 state、不取进程锁、默认不做真实账号探测——`authorization not probed` 是预期；可选 `--probe-auth` 显式发起一次经过 rclone 凭据路径的认证请求（默认仍不探测）。远程个人空间模式预检：

```bash
./drivebridge doctor --remote-url https://files.example.com --remote-token-file /absolute/private/agent-token --local-root /absolute/allowed/files
```

## 5. 注册 MCP 客户端（本机 stdio）

先把常用路径写进配置文件（一次即可，state/rclone/回存端点全部复用）：

```bash
./drivebridge config init && $EDITOR ~/.config/drivebridge/config.yaml
./drivebridge config show            # 核对每键来源
./drivebridge capabilities --json    # 确认工具面与空间
./drivebridge install --client claude-code   # 或 codex / generic
```

`install` 写入客户端注册（Claude Code user scope / Codex config.toml / 打印片段）；此后由客户端拉起 `drivebridge mcp --enabled --handoff`，配置就绪后无需再传 `--state/--rclone-config`。`--enabled/--handoff` 是"默认关闭的写入口开关"，设计上**只能**显式命令行、不进配置文件。不加 `--enabled` 时只有能力发现与回执查询。

## 6. Scaena 接线（可选：参考交接与回存）

Scaena 服务进程配置 `SCAENA_DRIVEBRIDGE_SOCKET`（state 目录下 `handoff.sock` 绝对路径）与 `SCAENA_DRIVEBRIDGE_ACTOR`，并显式启用已验证的参考生成 binding。完整合同与回滚说明见 Scaena 仓 `agent/scaena/docs/runtime/drivebridge.md`。

## 7. 小文件冒烟（真实账号第一证据）

在 local root 放一个小文件，经 MCP（或 `remote --action`）完成：`list` → `upload` → `stat` → `operation_get` → `download`，比对内容一致。命令经证据 runner 执行并落盘：

```bash
python3 scripts/check.py -- python3 scripts/mcp_smoke.py
```

真实账号冒烟另在 run 目录附一行说明（账号、空间、日期），token/URL 由 runner 统一脱敏。

## 8. 首个真实参考任务（可选：Scaena 闭环）

1. Agent 用 `upload` 产出稳定引用 `drivebridge://file/<id>`。
2. Scaena `createVideoProviderTask` 的 `reference_upload_ref` 填该引用；权利/人物/模型/费用门照常先过。
3. 任务 `succeeded` 且 `import_status=imported` 后 `save_output(space_id, submission_ref, idempotency_key)` 回存 OneDrive 私有副本。

排队有效期、火山区域抓取等运行时行为在首个任务中如实记录，不据此扩大默认开启。

## 9. 常见失败对照

| doctor 失败项 | 处置 |
|---|---|
| `state_dir_not_private` | `chmod 0700 <state>` |
| `rclone_config_missing` | 先 `./drivebridge auth` |
| `no_onedrive_remote` | 先 `./drivebridge auth --auto`（探测需要已落盘的 drive_id 与凭据） |
| `authentication_required` | 凭据失效：重跑 `./drivebridge auth`（换 `--remote` 名或先删旧节） |
| `rclone_remote_missing` | space-add 的 remote 名与 rclone 配置节名不一致 |
| `no_spaces` / `state_database_missing` | 先 `space-add` |
| `local_root_not_writable` | 修正目录或权限 |
| `ffprobe_missing` | 安装 ffprobe（视频参考必需） |
| `handoff_socket_unreachable` | 确认旧进程已停止后删除残留 socket |
| `credential_file_not_private` | `chmod 0600 <token-file>` |
| `scaena_unreachable` / `service_unreachable` | 核对服务地址与监听 |

## 10. 回滚

移除 MCP 客户端注册与 Scaena 环境变量，停止进程即可；OneDrive 文件、DriveBridge state 与任务回执全部保留，无资产迁移。
