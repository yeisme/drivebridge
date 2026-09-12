# 首次真实使用 runbook（本机创作闭环）

把 DriveBridge 从本地候选推进到真实账号激活。所有 OAuth 与账号步骤在**用户本人终端**完成；Agent 不代持凭据、不读取或打印配置。真实账号验收与 fixture 验证分开记录，不用本地结果宣称生产可用。

## 0. 前置条件

- Go 1.26；`ffprobe`（MP4 参考校验必需）
- 需要参考交接/成品回存时：本机 Scaena 服务（见第 6 步）
- 仅支持微软 global 云区域与 Linux/macOS

## 1. 微软 OAuth（用户终端）

```bash
go build -o drivebridge ./cmd/drivebridge
./drivebridge auth
```

在交互配置器中选择 OneDrive 完成登录；已有 rclone 配置可直接 `--rclone-config /absolute/path/rclone.conf`，无需重新登录。凭据只保存在 rclone 配置里，DriveBridge 不保存第二份 token。

## 2. 获取真实 drive-id 与 root-id

- **drive-id**：认证完成后 rclone 配置中该 remote 的 `drive_id =` 值（也可用 Graph Explorer 查询 `GET /me/drive`）。
- **root-id**：允许目录的真实 item ID，**不能**用显示名称或 `root` 占位符。用 Graph Explorer `GET /me/drive/root/children` 找到目标目录条目的 `id` 字段（Graph Explorer 用同一账号登录即可，无需管理员）。

## 3. 配置空间

```bash
./drivebridge space-add --space creative \
  --remote '<rclone-remote>' --drive-id '<drive-id>' --root-id '<folder-item-id>' \
  --local-root /absolute/path/creative-files \
  --max-bytes 1073741824 \
  --project '<scaena-project>' --actor '<scaena-actor>'
```

`--project/--actor` 仅成品回存需要；纯传输空间可省略。`--max-bytes` 按你的域/模型已验证上限设置。空间身份绑定后不可改绑。

## 4. 预检（新命令）

```bash
./drivebridge doctor --state ~/.config/drivebridge --rclone-config ~/.config/rclone/rclone.conf
```

按名输出 `pass/fail/info`，ready 以 0 退出；支持 `--json/--agent/--events/--explain`。doctor 只读：不创建 state、不取进程锁、不做真实账号探测——`authorization not probed` 是预期。远程个人空间模式预检：

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
| `rclone_remote_missing` | space-add 的 remote 名与 rclone 配置节名不一致 |
| `no_spaces` / `state_database_missing` | 先 `space-add` |
| `local_root_not_writable` | 修正目录或权限 |
| `ffprobe_missing` | 安装 ffprobe（视频参考必需） |
| `handoff_socket_unreachable` | 确认旧进程已停止后删除残留 socket |
| `credential_file_not_private` | `chmod 0600 <token-file>` |
| `scaena_unreachable` / `service_unreachable` | 核对服务地址与监听 |

## 10. 回滚

移除 MCP 客户端注册与 Scaena 环境变量，停止进程即可；OneDrive 文件、DriveBridge state 与任务回执全部保留，无资产迁移。
