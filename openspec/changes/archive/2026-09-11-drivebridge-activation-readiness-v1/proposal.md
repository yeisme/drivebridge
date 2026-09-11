## Why

DriveBridge v1 是本地候选：真实首次使用路径散落在 README 各节，激活前无法用一条命令确认就绪。需要只读预检与首用 runbook，把"本地候选"推进到"本人可激活"。

## What Changes

新增 `doctor` 只读预检子命令（本机与远程双模式）与 `docs/activation-local.md` 首次真实使用 runbook。不新增写入口、不探测真实账号授权、不改变既有传输与交接合同。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-local-files`：新增 Preflight readiness reporting 需求（只读报告、按名失败、凭据不进输出、远程模式区分 not probed）。

## Impact

doctor 不创建 state、不取进程锁、以只读方式读取 state.sqlite，可与运行中的 `mcp` 并发执行。真实账号激活（rclone OAuth、Scaena 实机 wiring、OneDrive 账号）仍由用户本人执行；本 change 只准备就绪面与文档，不扩大任何凭据或默认开启任何入口。
