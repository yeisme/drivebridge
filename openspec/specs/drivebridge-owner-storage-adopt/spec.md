# drivebridge-owner-storage-adopt Specification

## Purpose
TBD - created by archiving change drivebridge-owner-storage-adopt-v1. Update Purpose after archive.
## Requirements
### Requirement: Adopt 零拷贝挂接同一 local 或同一 S3 位置

DriveBridge SHALL 提供 `storage adopt --kind local|s3`，把 owner 已有的本地目录或 S3 bucket/prefix 注册为空间。local SHALL 是该目录的零拷贝 alias 视图（复用既有 local kind 空间注册）。s3 SHALL 指向同一 remote 与 `<bucket>/<prefix>` 对象集合，SHALL NOT 复制桶内字节。成功信封（`--json`）SHALL 包含 `consumer`、`kind`、`space`、`location`、`copied=false`。Adopt 记录 SHALL 由 CLI/application service 写入，含 consumer、kind、location、space、可选 connection_id 与幂等键。SHALL NOT 删除 `storage add`。

#### Scenario: 托管本地 vault
- **WHEN** `drivebridge storage adopt --kind local --consumer pinax --space pinax-vault --root <绝对目录>`
- **THEN** 空间指向该目录，list 看到同一批文件，磁盘上不出现第二份副本，信封 `copied=false`

#### Scenario: 托管已有 S3 前缀
- **WHEN** `drivebridge storage adopt --kind s3 --consumer eikona --space eikona-exports --remote <rclone-s3-remote> --remote-path '<bucket>/<prefix>' --local-root <绝对目录>`
- **THEN** DriveBridge 列出同一前缀对象且不复制该桶；信封 `copied=false`

#### Scenario: 加法保留 storage add
- **WHEN** 调用既有 `storage add --kind local|s3`
- **THEN** 行为与 adopt 落地前一致

### Requirement: 幂等与位置冲突

同一幂等键且同一 location SHALL 复用已有 adopt 并成功。同一幂等键且不同 location SHALL 拒绝并返回 `adopt_mismatch`，SHALL NOT 创建部分空间。缺省幂等键 SHALL 稳定派生自 consumer/kind/space，使重复执行同一命令可复用。

#### Scenario: 同键同位置复用
- **WHEN** 以同一 `--idempotency-key` 对同一目录或同一 bucket/prefix 再次 adopt
- **THEN** 成功复用既有记录与空间，`copied=false`

#### Scenario: 同键不同位置拒绝
- **WHEN** 同一幂等键声明了不同的绝对目录或不同的 bucket/prefix
- **THEN** 返回 `adopt_mismatch`，不把该空间改绑到新位置

### Requirement: 本节点只 adopt local 与 s3

本节点 `storage adopt` 的 `--kind` SHALL 仅为 `local` 或 `s3`。未知 kind SHALL 返回 `storage_kind_unknown`。onedrive、gdrive 与逃生舱 kind SHALL 返回 `unsupported`（或等价稳定码），并指向既有 `drivebridge auth --auto` / `setup` 或 `storage add`。

#### Scenario: 拒绝个人网盘与逃生舱
- **WHEN** `storage adopt --kind onedrive|gdrive|openlist|webdav|smb|sftp|alias`
- **THEN** 以稳定错误拒绝，并提示改走 auth/setup 或 storage add

#### Scenario: 未知 kind
- **WHEN** `storage adopt --kind` 不在目录中
- **THEN** 返回 `storage_kind_unknown`

### Requirement: Adopt 状态可查询

CLI SHALL 提供 `storage adopt status`（或等价投影）。成功 adopt 信封与 status SHALL 能列出 consumer、kind、space、location、`copied=false`。空间列表 MAY 加法附带 adopt 事实。

#### Scenario: status 列出已 adopt 空间
- **WHEN** 至少一个成功 adopt 之后运行 `storage adopt status --json`
- **THEN** 结果含对应 consumer/kind/space/location 且 `copied=false`

