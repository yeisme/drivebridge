## Why

Finder 需要把文件复制成新身份，而现有 `POST /api/v1/archive` 是产物归档：排除 CAS 秒传，且语义不是文件管理器复制。网页不能在客户端拼装覆盖或假复制。

## What Changes

- 新增 `POST /api/v1/files/{id}/copy`：固定源版本、目标空间与路径、`conflict=rename|fail`（默认 rename）、必填 `Idempotency-Key`。
- 同 data owner 受管 S3 目标优先 CAS 秒传；未命中则服务端转存为可 resume 的 `kind=copy` 操作。
- 永不覆盖已有路径。`rename` 使用 `name (2).ext`；`fail` 返回 `name_conflict`。
- SDK 增加 `Copy`。不改 move/archive/trash/purge 路径或错误码含义。MCP 工具本切片不加。

## Capabilities

### New Capabilities
- `file-copy`：受管文件复制合同。

### Modified Capabilities
- `drivebridge-personal-service`：增加 copy 操作种类与 resume。

## Impact

`split-owner`：canonical 复制规则在本服务。网页/本机 view 只调用合同。Agent 仍走 archive；不能 purge。文件夹递归复制不在本切片。
