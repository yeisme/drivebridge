## Why

根包 `drivebridge-consumer-binding-v1` 要求：各项目原有的 local / S3 仍是 canonical 写入路径，DriveBridge 以 **零拷贝 adopt** 挂接同一目录或同一桶前缀，提供浏览、钉版本与交接。DB-1-01 已标出 `local`/`s3` 为可管理默认存储；本 change 落地 `storage adopt`、adopt 记录与写入围栏，不复制字节、不改写 owner 元数据。

## What Changes

- 新增 `drivebridge storage adopt --kind local|s3`：把 owner 已有本地目录或 S3 bucket/prefix 注册为 generic 空间（local 复用 alias 零拷贝视图，S3 指向同一对象集合）。
- CLI/application service 写入 adopt 记录：consumer、kind、location、space、可选 connection_id 与幂等键。同键同位置复用成功；同键不同位置拒绝 `adopt_mismatch`。
- 已 adopt 空间的变异文件操作（upload/delete/mkdir/move/rename）若相对路径落在 owner 保护路径，返回稳定码 `owner_protected`；list/stat 仍允许。
- `storage adopt status` 与成功信封给出 consumer/kind/space/location/`copied=false`。doctor 对 S3 adopt 诚实声明不自动识别 Capsa 密文（`opaque_encrypted` 为 follow-up）。
- 本节点 adopt 只接受 `local` 与 `s3`。onedrive/gdrive/逃生舱拒绝并指向既有 `auth`/`setup` 或 `storage add`。
- **不删除** `storage add`；不回归 `tier` / `bind_default` / `bind_default_kinds`。

## Capabilities

### New Capabilities
- `drivebridge-owner-storage-adopt`：零拷贝挂接 owner 默认 local/S3、adopt 记录、写入围栏与 doctor 诚实性。

### Modified Capabilities
- `drivebridge-local-files`：已 adopt 空间的变异文件操作对 owner 保护路径返回 `owner_protected`（list/stat 不变）。

## Impact

Owner：`mcp/drivebridge`。触及 `storage adopt` CLI、bridge adopt 记录与路径围栏、recovery 新码、recipe/错误码文档。不改 Pinax/Scaena/Eikona，不实现 consumer bind，不接真实云账号。
