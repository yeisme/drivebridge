## Why

根包 `pinax-drivebridge-mounted-vault-v1` 要求对象存储上的 Pinax vault 走本机路径，而不是让 Pinax 适配 S3。本机已有只读 `mount webdav`（端口，不是目录）。Pinax 需要 `--vault-root` 上的可写目录，且 `.pinax` 不得落到对象存储。

## What Changes

- 新增 `drivebridge mount vault --preset pinax --space --vault-root --writable`：把空间内容树叠加进本机 vault 根（内容符号链接，控制面留在本机）。
- 新增 `drivebridge preset pinax-vault`：对已有空间执行上述叠加，打印 Pinax 本地下一步命令，不 exec `pinax`。
- `mount status` / capabilities 加法字段：`preset`、`vault_root`、`overlay`、`adapter`。
- 新错误码：`mount_path_adapter_missing`、`vault_root_not_local`、`mount_overlay_conflict`、`preset_prefix_is_capsa`、`content_unmounted`。
- 本切片路径：`kind=local` alias 零 FUSE；S3（rclone `type=s3`）exec 本机 `rclone mount`（默认二进制不链 libfuse）。其它远程或 rclone/FUSE 不可用时 `mount_path_adapter_missing`，不把 WebDAV 端口冒充成功。
- Agent 仍不能启动挂载。不链 libfuse。不删除 `mount webdav`。

## Capabilities

### New Capabilities

- `drivebridge-pinax-vault-mount`：pinax 预设路径叠加、控制面本地、status/能力键与错误码。

### Modified Capabilities

- `drivebridge-mount-surface`：`mount vault` 与 `preset pinax-vault` 为加法；WebDAV 默认只读原义不变。

## Impact

Owner：`cli/drivebridge`。触及 `cmd/drivebridge` 挂载/预设、`internal/bridge` overlay、recovery 码、testdata、docs。Pinax 实现在 `cli/pinax` 的 `pinax-mounted-vault-local-v1`。
