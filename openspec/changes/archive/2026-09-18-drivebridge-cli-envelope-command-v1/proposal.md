## Why

失败信封的 `command` 字段取叶命令名（`drivebridge.add`/`drivebridge.probe`），成功信封取 action（`drivebridge.space-add`/`drivebridge.storage.probe`）。机器消费方在同一次调用的成功/失败路径看到两个身份，无法按 command 稳定路由。storage-onboarding review（2026-09-17）已确认该缺陷影响全部多词命令。

## What Changes

- `PersistentPreRunE` 从 `cmd.CommandPath()` 派生 `c.command`（去掉 `drivebridge ` 前缀、空格转点），并保留两个既有连字符 action 映射：`space.add→space-add`、`storage.add→storage-add`。失败与成功信封 command 从此一致。
- 归一两个 mount 子命令的成功 action：`status→mount.status`、`stop→mount.stop`（当前失败=成功的裸名，但与全树 dotted 命名不一致；无测试或文档钉住旧值）。
- `deviceauth.go` 两处 `SaveConfig()`（无返回值、磁盘失败被吞）改为 `Data().Save()` 并传播错误。
- 快速入门与命令树行同步（`space-add` 已弃用 → `space add`；命令树标注为常用子集）。

契约影响：各命令**失败**信封的 command 值从叶名变为全限定 action（如 `drivebridge.space-add`）；成功值除 mount status/stop 归一外不变。本仓 testscript 不钉失败 command 旧值；接口 pre-1.0 未发布。回滚 = 还原 `PersistentPreRunE` 一行与 mount 两个 action 字符串。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-local-files`：Machine-mode failure contract 增加失败/成功 command 同一性要求。

## Impact

Owner：`mcp/drivebridge`。验证：`python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、本包 strict 校验。
