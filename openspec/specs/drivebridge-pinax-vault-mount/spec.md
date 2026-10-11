# drivebridge-pinax-vault-mount Specification

## Purpose
TBD - created by archiving change drivebridge-pinax-vault-mount-v1. Update Purpose after archive.
## Requirements
### Requirement: pinax 预设 SHALL 把空间呈现为本机 vault 路径

`drivebridge mount vault --preset pinax --space <id> --vault-root <abs> --writable` 与 `drivebridge preset pinax-vault --space <id> --vault-root <abs>` SHALL 在成功时让 `--vault-root` 成为本机绝对目录，用户内容树可经普通文件系统访问。SHALL NOT exec `pinax`。SHALL NOT 仅返回 WebDAV URL 却报告 pinax-vault 成功。Agent token SHALL NOT 能启动二者。

#### Scenario: 本地空间叠加成功
- **WHEN** 已有 `kind=local` 空间且用户运行 `drivebridge preset pinax-vault --space docs --vault-root /abs/vault`
- **THEN** `/abs/vault` 存在为本机目录，内容树（至少 `notes`）经符号链接指向空间内容根
- **AND** 成功信封含 `preset=pinax-vault`、`writable=true`、`vault_root`
- **AND** SHALL NOT 调用 `pinax` 二进制

#### Scenario: S3 空间经 rclone mount 叠加
- **WHEN** 空间的 rclone remote `type=s3` 且本机 `rclone mount` 成功
- **THEN** 内容根是 state 下的 rclone 挂载点，叠加进 `--vault-root`
- **AND** 成功信封 `adapter=pinax-vault-rclone`、`preset=pinax-vault`
- **AND** SHALL NOT exec `pinax`，SHALL NOT 把 WebDAV 端口写成 `vault_root`

#### Scenario: 缺少路径适配器
- **WHEN** 空间不是可本地解析的 alias，且 rclone remote 不是 s3 或本机没有可用 `rclone mount`
- **THEN** 失败为 `mount_path_adapter_missing`
- **AND** SHALL NOT 把 WebDAV 监听地址报告为 `vault_root`

### Requirement: 控制面 SHALL 留在本机

叠加 SHALL 忽略并永不链接 `.pinax/**`、SQLite/WAL、token、`.git/**`。远程或内容根已有 `.pinax` SHALL 不覆盖 vault 根已有控制面。DriveBridge CAS 目录 SHALL NOT 作为 `--vault-root`。

#### Scenario: 本机 SQLite 不进内容根
- **WHEN** 预设成功后用户在 `--vault-root/.pinax` 创建文件
- **THEN** 空间内容根 listing SHALL NOT 因该叠加而新增 `.pinax`
- **AND** `mount stop` 后本机 `.pinax` 仍在

#### Scenario: Capsa 前缀拒绝
- **WHEN** 内容根含 Capsa `protocol.json` 布局
- **THEN** 失败为 `preset_prefix_is_capsa`

### Requirement: status 加法字段诚实

`mount status --json` SHALL 对 pinax 预设报告 `preset`、`vault_root`、`overlay`、`adapter`、`alive`。capabilities 的 `mount.start` 仍为 `cli_only`，并区分 WebDAV 与路径预设。同一 state 第二挂载仍为 `mount_already_running`。

#### Scenario: 停叠加保留两端
- **WHEN** `drivebridge mount stop` 且当前是 pinax 预设
- **THEN** vault 根上的内容符号链接被移除
- **AND** 空间内容根与本机 `.pinax` 均保留

