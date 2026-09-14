## Context

rclone v1.70.3 `backend/onedrive` 内置公共 client id（`rcloneClientID`）；oauthutil 不提供 device flow 助手，但 token JSON 落盘 rclone 配置后，刷新由 oauthutil 现有机制接管（本项目 `token()` 已经借道 `About()` 触发刷新）。Graph helper `graph()` 已具备发现请求能力。

## Decisions

- **device-code 流自实现**（约百行）：`POST https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode`（scope=`Files.ReadWrite offline_access`，client_id=rclone 公共 id）→ 打印 `verification_uri`+`user_code`（仅这两项与过期时间；不打印 token）→ 按 `interval` 轮询 `/token` 至 `authorization_pending` 结束 → 把 `oauth2.Token` JSON 写入 rclone 配置新节（configfile API 已安装；节名默认 `onedrive`，冲突则 `onedrive-2` 递增），同时写入 `type = onedrive`。仅支持 global 云（与现有一致，consumers 端点固定）。
- **drive-id 自动发现**：auth 成功即用 token 调 `GET /me/drive`，打印并缓存 drive-id（state 目录 `last-discovery.json`，权限 0600）；`space-add` 在未显式给 `--drive-id` 时读取该缓存。
- **root 交互选择**：`space-add --auto`（或未给 `--root-id` 且终端可交互时提示确认）→ `GET /me/drive/root/children` 过滤 folder → 编号菜单（名称+item id 前缀+子项数）→ 选择写入 `root-id`；`--root-name <name>` 为非交互等价（唯一匹配，多匹配/无匹配报错，不做猜测）。仍接受显式 `--drive-id/--root-id` 覆盖一切（幂等路径不变）。
- **auth --status**：读 rclone 配置节 token 的 expiry 做本地判断（`expired_at` 相对当前时间），输出 `valid|expired|missing|refreshable_unknown`；`--probe` 才做真实 `About` 调用。诚实就绪：无 probe 不宣称账号可用。
- **doctor**：新增可选 `--probe-auth`（执行一次真实 About+token 有效性），默认保持 `authorization not probed`。
- **错误面**：device flow 失败/过期/取消、发现失败、菜单 EOF 均为固定码（`device_flow_failed`、`discovery_failed`、`root_selection_aborted`），不输出微软原始响应体。
- **交互边界**：菜单仅在 CLI 终端（`os.Stdin` 是 tty）；MCP/管道环境无 `--root-name`/显式 ID 时直接报 `root_selection_aborted`，不阻塞等待。

## Interfaces

`cmd/drivebridge/auth.go`：`runAuth`（device flow + 发现）、`authStatus`；`internal/bridge/onedrive.go` 增 `DefaultDrive(ctx, remote)`、`RootChildren(ctx, remote)` 两个发现助手（走既有 token/refresh 管道）。`main.go` 装配 `--rclone-interactive/--status/--probe/--auto/--root-name` 旗标。

## Verification / Rollback

fixture：fake device-code/token 端点（httptest 注入端点覆盖）+ 既有 fake Graph 提供 `/me/drive`、`/root/children`；testscript 全链（auth→发现缓存→space-add --root-name→doctor ready）；`--status` 三态；真实账号登录由用户本人终端验收，fixture 不冒充。回滚：默认 auth 回到 rclone 交互配置器即可，无状态迁移（last-discovery.json 可删）。

## 去向

实现波：按本设计落地 + 重写 `docs/activation-local.md` §1-3 + conformance needle 同步（若 activation-local 断言词变化）。
