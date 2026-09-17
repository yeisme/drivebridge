package recovery

// Registry is the single source of error-code recovery hints, shared by the
// CLI human output and the MCP error payload; cmd/drivebridge keeps the
// docs/error-codes.md table in sync with this registry.
type Doc struct {
	HintEN string // one-line English recovery hint shown in CLI output
	DocZH  string // Chinese trigger + recovery sentence for the docs table
}

var Registry = map[string]Doc{
	"aigora_access_denied": {
		HintEN: "The Aigora exchange rejected the configured key; fix the Aigora key configuration on the adapter host and retry the original command.",
		DocZH:  "Aigora 交换拒绝配置的 Key：在适配器宿主修正 Aigora Key 配置后重试原命令，不切换直连或旧凭据。",
	},
	"credential_file_not_private": {
		HintEN: "The credential file must be a regular file with no group/other access; chmod 0600 and retry; its content was not read.",
		DocZH:  "凭据文件必须是 0600 的普通文件：chmod 0600 后重试；文件内容未被读取。",
	},
	"aigora_unavailable": {
		HintEN: "The configured Aigora origin is unreachable; restore connectivity to it and retry the original command.",
		DocZH:  "Aigora 端点不可达：恢复与所配置 Aigora origin 的连通后重试原命令；不会回退长期 Key 直连。",
	},
	"invalid_service_context": {
		HintEN: "The Aigora exchange returned an invalid short-lived context; report to the deployment operator instead of retrying legacy credentials.",
		DocZH:  "Aigora 交换返回非法短期凭证：报给部署运维核对，不要回退旧凭据重试。",
	},
	"service_admission_required": {
		HintEN: "This hosted write path is not enabled for the current identity; stop and request admission through the owning service.",
		DocZH:  "统一身份写路径未开通准入：停止并在所属服务申请准入，不能替换凭据或另走直连。",
	},
	"state_locked": {
		HintEN: "A registered MCP client (drivebridge mcp) holds this state's process lock; use recent/operation which read a snapshot, or stop the client and retry.",
		DocZH:  "有已注册的 MCP 客户端（drivebridge mcp）持有该 state 的进程锁。用 recent/operation（快照读）查看状态，或停掉客户端后重试。",
	},
	"probe_failed": {
		HintEN: "The remote could not be listed; check the remote name, credentials and remote-path, then re-run 'drivebridge storage probe'.",
		DocZH:  "远端无法列举：核对 remote 名称、凭据与 remote-path 后重跑 drivebridge storage probe。",
	},
	"storage_kind_unknown": {
		HintEN: "Unknown --kind for 'drivebridge storage add'; run 'drivebridge storage kinds' for the supported list.",
		DocZH:  "storage add 的 --kind 不在支持列表：先运行 drivebridge storage kinds 查看。",
	},
	"storage_missing_flag": {
		HintEN: "Storage add is missing required flags for this kind; the failure names them, or run 'drivebridge storage kinds'.",
		DocZH:  "storage add 缺少该类型必填旗标：按提示补齐，或用 drivebridge storage kinds 查看必填项。",
	},
	"authentication_required": {
		HintEN: "Reconnect using drivebridge auth --auto; then resume the original operation.",
		DocZH:  "需要重新认证：运行 drivebridge auth --auto（--auto 空间必须带 --auto 发现根），再恢复原操作。",
	},
	"source_changed": {
		HintEN: "Use the original source for recovery; changed content needs a new idempotency key.",
		DocZH:  "恢复必须用原源文件；源内容已变化时需换新的幂等键重新传输。",
	},
	"content_mismatch": {
		HintEN: "Bytes did not match the recorded digest; retry from the original source with a new idempotency key.",
		DocZH:  "字节与记录摘要不符：从原始来源重试并换新的幂等键。",
	},
	"file_version_changed": {
		HintEN: "List the folder again and select a current reference; the pinned version was changed remotely.",
		DocZH:  "钉住的版本已被远端改动：重新 list 该目录并选取当前引用。",
	},
	"target_conflict": {
		HintEN: "Choose a different output path; the existing file was preserved.",
		DocZH:  "目标路径已有文件（原文件未动）：换一个输出路径。",
	},
	"resume_required": {
		HintEN: "Inspect the original operation, restore connectivity, then resume it without regenerating content.",
		DocZH:  "传输被中断（非失败）：查 operation_get 后用 resume 原操作续传，绝不重新生成内容。",
	},
	"connection_lost": {
		HintEN: "Connectivity dropped mid-transfer; inspect the original operation and resume it.",
		DocZH:  "传输中断网：查原操作后 resume 续传。",
	},
	"graph_unreachable": {
		HintEN: "The OneDrive endpoint is unreachable; restore connectivity, then resume the original operation.",
		DocZH:  "OneDrive 端点不可达：恢复网络后 resume 原操作。",
	},
	"throttled": {
		HintEN: "The provider throttled the transfer; wait, then resume the original operation.",
		DocZH:  "供应商限流：等待后 resume 原操作。",
	},
	"quota_exceeded": {
		HintEN: "Free space or adjust the storage quota before resuming the original operation.",
		DocZH:  "空间配额已满：清理空间或调整 max-bytes 后 resume 原操作。",
	},
	"disabled": {
		HintEN: "The service started without --enabled; write tools are gated. Restart drivebridge mcp with --enabled.",
		DocZH:  "服务未带 --enabled 启动，全部写工具被门禁。重启 mcp 时加 --enabled。",
	},
	"symlink_rejected": {
		HintEN: "Symlinks are refused by default; pass a real file or enable follow-symlinks on the space.",
		DocZH:  "默认拒绝符号链接防越界：改传真实文件，或在空间上显式开启 follow-symlinks。",
	},
	"path_not_allowed": {
		HintEN: "Paths are confined to the configured root; list first, then act on listed entries.",
		DocZH:  "路径被限制在空间根内且不可用绝对路径/..：先 list 再对列出的条目操作。",
	},
	"remote_outside_space": {
		HintEN: "The reference points outside the space root; list the space and use a current reference.",
		DocZH:  "引用指向空间根之外：重新 list 空间并使用当前引用。",
	},
	"remote_not_found": {
		HintEN: "The remote no longer exists, or for S3-family backends the remote-path is missing its bucket segment (<bucket>/<prefix>).",
		DocZH:  "远端不存在；S3 族 backend 的 remote-path 第一段必须是桶名（<bucket>/<prefix>），只写前缀会被当成桶。",
	},
	"remote_exists": {
		HintEN: "A credential already exists under that rclone section name; re-auth under a new name or remove the old section.",
		DocZH:  "该 rclone 节名已有凭据：换名重试或先删除旧节。",
	},
	"invalid_space": {
		HintEN: "The space identifier or its configuration is invalid; check space list and doctor.",
		DocZH:  "空间标识或配置非法（含 max-bytes 未设置）：用 space list 与 doctor 核对。",
	},
	"cursor_ahead": {
		HintEN: "The requested cursor is ahead of the feed; re-sync from the current position (since=0 returns the position, not history).",
		DocZH:  "请求的游标领先于变更流：从当前位置重新同步（since=0 只返回当前位置，不回放历史）。",
	},
	"file_trashed": {
		HintEN: "The file is in the space trash; restore it first.",
		DocZH:  "文件已在回收站：先 restore 再访问。",
	},
	"file_not_trashed": {
		HintEN: "Purge requires the file to be trashed first; trash, then purge --i-am-the-owner.",
		DocZH:  "永久删除必须先移入回收站：先 trash，再 purge --i-am-the-owner（仅 owner 终端）。",
	},
	"reference_not_found": {
		HintEN: "The stable reference no longer resolves; list the folder and pin a current reference.",
		DocZH:  "稳定引用已不可解析：重新 list 目录并钉住当前引用。",
	},
	"idempotency_conflict": {
		HintEN: "The same idempotency key was used for different content; use a new key for the new source.",
		DocZH:  "同一幂等键被用于不同内容：为新的源内容换新键。",
	},
	"operation_closed": {
		HintEN: "The operation already reached a terminal state; start a new operation for new work.",
		DocZH:  "操作已达终态：新工作请发起新操作。",
	},
	"transfer_capacity_unavailable": {
		HintEN: "Two transfers are already active; wait for one to finish (operation_get), then retry.",
		DocZH:  "同时最多两个传输：等待任一完成（operation_get）后重试。",
	},
	"transfer_failed": {
		HintEN: "The transfer failed after retries; inspect the original operation, fix the cause, then resume.",
		DocZH:  "传输重试后仍失败：查原操作、修复原因后 resume。",
	},
	"root_selection_conflict": {
		HintEN: "Root selection flags (--auto/--root-name/--root-path/--drive-id) are mutually exclusive; pass exactly one.",
		DocZH:  "根选择旗标（--auto/--root-name/--root-path/--drive-id）互斥：只能传一个。",
	},
	"remote_selection_required": {
		HintEN: "No remote chosen; pass --remote or run setup interactively in a terminal.",
		DocZH:  "未选择 remote：传 --remote，或在真实终端里跑交互式 setup。",
	},
	"folder_path_required": {
		HintEN: "The interactive folder path is required in a terminal; piped runs must use explicit flags.",
		DocZH:  "交互式文件夹路径只能在终端输入；管道运行必须用显式旗标。",
	},
	"unknown_config_key": {
		HintEN: "The configuration file contains a key this version does not know; remove or correct it (config show lists sources).",
		DocZH:  "配置文件含未知键（config show 可看来源与键名）：修正或删除该键。",
	},
	"config_write_failed": {
		HintEN: "The configuration file could not be written; check directory permissions and retry.",
		DocZH:  "配置文件写入失败：检查目录权限后重试。",
	},
	"registration_conflict": {
		HintEN: "The client registration conflicts with an existing entry; remove the old registration or choose another name.",
		DocZH:  "客户端注册与已有条目冲突：删除旧注册或换名。",
	},
	"handoff_socket_unreachable": {
		HintEN: "The handoff socket is unreachable; ensure the MCP server runs with --handoff (a stale socket is cleaned automatically).",
		DocZH:  "handoff socket 不可达：确认 MCP 服务带 --handoff 运行（陈旧 socket 会被自动清理）。",
	},
	"output_adapter_unavailable": {
		HintEN: "save_output needs --scaena-url at MCP start; restart the server with it configured.",
		DocZH:  "save_output 需要 MCP 启动时带 --scaena-url：配置后重启服务。",
	},
	"secure_handoff_unavailable": {
		HintEN: "prepare_reference needs --handoff at MCP start and a space bound to a project/actor.",
		DocZH:  "prepare_reference 需要 MCP 启动带 --handoff，且空间已绑定 project/actor。",
	},
	"insights_unavailable": {
		HintEN: "The insights store is unavailable; run doctor to check the state directory.",
		DocZH:  "insights 存储不可用：跑 doctor 检查 state 目录。",
	},
	"listen_not_allowed": {
		HintEN: "Non-loopback listen requires the explicit --lan flag; the default mount is loopback-only.",
		DocZH:  "挂载默认仅回环：非回环 listen 必须显式加 --lan。",
	},
	"mount_already_running": {
		HintEN: "This state already has a mount; run mount status, or stop it before starting another.",
		DocZH:  "同一 state 只允许一个挂载：先 mount status 查看，或 stop 后再挂。",
	},
	"mount_listener_unavailable": {
		HintEN: "The requested listen address is unavailable; choose a free loopback port and retry.",
		DocZH:  "监听地址不可用：换一个空闲回环端口重试。",
	},
	"mount_stop_unavailable": {
		HintEN: "The mount process cannot be signaled from this platform; close its terminal instead.",
		DocZH:  "本平台无法向挂载进程发停止信号：直接关闭其终端。",
	},
	"unsupported": {
		HintEN: "This capability is not in the default build; use the documented WebDAV fallback.",
		DocZH:  "默认构建不含该能力（如内核 FUSE）：改用 mount webdav，或把 rclone mount 指到 WebDAV URL。",
	},
}

// Hint returns the English recovery hint for a code, falling back to the
// generic guidance that matches the core discipline (inspect before retry).
func Hint(code string) string {
	if d, ok := Registry[code]; ok {
		return d.HintEN
	}
	return "Inspect the original operation and its error before retrying; existing files are preserved."
}
