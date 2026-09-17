# DriveBridge 错误码参考

`drivebridge` 的全部输出模式（默认人读摘要、`--agent`、`--json`、`--events`、`--explain`）都以稳定错误码报告失败；人读模式同时给出一条恢复建议。本表由 `cmd/drivebridge/codes.go` 注册表生成（`codes_test.go` 保证两者不漂移）。

通则：

- 恢复传输前先 `operation_get` / `drivebridge operation` 查原操作，再 `resume`；绝不重新生成内容。
- 未在本表的码（新版本新增或内部码）沿用通用建议：先查原操作再重试，既有文件不会被动。
- 持锁期间（`state_locked`）：`recent`/`operation`/`doctor` 走只读快照照常可用；写命令需先停 MCP 客户端。
- `--json`/`--events` 失败时也保证输出合法信封（`error.code` 为真实错误码），诊断信息只进 stderr。

| 错误码 | 触发与恢复 |
| --- | --- |
| `aigora_access_denied` | Aigora 交换拒绝配置的 Key：在适配器宿主修正 Aigora Key 配置后重试原命令，不切换直连或旧凭据。 |
| `aigora_unavailable` | Aigora 端点不可达：恢复与所配置 Aigora origin 的连通后重试原命令；不会回退长期 Key 直连。 |
| `authentication_required` | 需要重新认证：运行 drivebridge auth --auto（--auto 空间必须带 --auto 发现根），再恢复原操作。 |
| `config_write_failed` | 配置文件写入失败：检查目录权限后重试。 |
| `connection_lost` | 传输中断网：查原操作后 resume 续传。 |
| `content_mismatch` | 字节与记录摘要不符：从原始来源重试并换新的幂等键。 |
| `credential_file_not_private` | 凭据文件必须是 0600 的普通文件：chmod 0600 后重试；文件内容未被读取。 |
| `cursor_ahead` | 请求的游标领先于变更流：从当前位置重新同步（since=0 只返回当前位置，不回放历史）。 |
| `disabled` | 服务未带 --enabled 启动，全部写工具被门禁。重启 mcp 时加 --enabled。 |
| `file_not_trashed` | 永久删除必须先移入回收站：先 trash，再 purge --i-am-the-owner（仅 owner 终端）。 |
| `file_trashed` | 文件已在回收站：先 restore 再访问。 |
| `file_version_changed` | 钉住的版本已被远端改动：重新 list 该目录并选取当前引用。 |
| `folder_path_required` | 交互式文件夹路径只能在终端输入；管道运行必须用显式旗标。 |
| `graph_unreachable` | OneDrive 端点不可达：恢复网络后 resume 原操作。 |
| `handoff_socket_unreachable` | handoff socket 不可达：确认 MCP 服务带 --handoff 运行（陈旧 socket 会被自动清理）。 |
| `idempotency_conflict` | 同一幂等键被用于不同内容：为新的源内容换新键。 |
| `insights_unavailable` | insights 存储不可用：跑 doctor 检查 state 目录。 |
| `invalid_service_context` | Aigora 交换返回非法短期凭证：报给部署运维核对，不要回退旧凭据重试。 |
| `invalid_space` | 空间标识或配置非法（含 max-bytes 未设置）：用 space list 与 doctor 核对。 |
| `listen_not_allowed` | 挂载默认仅回环：非回环 listen 必须显式加 --lan。 |
| `mount_already_running` | 同一 state 只允许一个挂载：先 mount status 查看，或 stop 后再挂。 |
| `mount_listener_unavailable` | 监听地址不可用：换一个空闲回环端口重试。 |
| `mount_stop_unavailable` | 本平台无法向挂载进程发停止信号：直接关闭其终端。 |
| `operation_closed` | 操作已达终态：新工作请发起新操作。 |
| `output_adapter_unavailable` | save_output 需要 MCP 启动时带 --scaena-url：配置后重启服务。 |
| `path_not_allowed` | 路径被限制在空间根内且不可用绝对路径/..：先 list 再对列出的条目操作。 |
| `probe_failed` | 远端无法列举：核对 remote 名称、凭据与 remote-path 后重跑 drivebridge storage probe。 |
| `quota_exceeded` | 空间配额已满：清理空间或调整 max-bytes 后 resume 原操作。 |
| `reference_not_found` | 稳定引用已不可解析：重新 list 目录并钉住当前引用。 |
| `registration_conflict` | 客户端注册与已有条目冲突：删除旧注册或换名。 |
| `remote_exists` | 该 rclone 节名已有凭据：换名重试或先删除旧节。 |
| `remote_not_found` | 远端不存在；S3 族 backend 的 remote-path 第一段必须是桶名（<bucket>/<prefix>），只写前缀会被当成桶。 |
| `remote_outside_space` | 引用指向空间根之外：重新 list 空间并使用当前引用。 |
| `remote_selection_required` | 未选择 remote：传 --remote，或在真实终端里跑交互式 setup。 |
| `resume_required` | 传输被中断（非失败）：查 operation_get 后用 resume 原操作续传，绝不重新生成内容。 |
| `root_selection_conflict` | 根选择旗标（--auto/--root-name/--root-path/--drive-id）互斥：只能传一个。 |
| `secure_handoff_unavailable` | prepare_reference 需要 MCP 启动带 --handoff，且空间已绑定 project/actor。 |
| `service_admission_required` | 统一身份写路径未开通准入：停止并在所属服务申请准入，不能替换凭据或另走直连。 |
| `source_changed` | 恢复必须用原源文件；源内容已变化时需换新的幂等键重新传输。 |
| `state_locked` | 有已注册的 MCP 客户端（drivebridge mcp）持有该 state 的进程锁。用 recent/operation（快照读）查看状态，或停掉客户端后重试。 |
| `storage_kind_unknown` | storage add 的 --kind 不在支持列表：先运行 drivebridge storage kinds 查看。 |
| `storage_missing_flag` | storage add 缺少该类型必填旗标：按提示补齐，或用 drivebridge storage kinds 查看必填项。 |
| `symlink_rejected` | 默认拒绝符号链接防越界：改传真实文件，或在空间上显式开启 follow-symlinks。 |
| `target_conflict` | 目标路径已有文件（原文件未动）：换一个输出路径。 |
| `throttled` | 供应商限流：等待后 resume 原操作。 |
| `transfer_capacity_unavailable` | 同时最多两个传输：等待任一完成（operation_get）后重试。 |
| `transfer_failed` | 传输重试后仍失败：查原操作、修复原因后 resume。 |
| `unknown_config_key` | 配置文件含未知键（config show 可看来源与键名）：修正或删除该键。 |
| `unsupported` | 默认构建不含该能力（如内核 FUSE）：改用 mount webdav，或把 rclone mount 指到 WebDAV URL。 |
