## Why

Server 已有随机 file/version、上传前 SHA-256 和 Complete 后复读，但对象键仍是 `<instance>/<owner>/<space>/<version>`，相同内容会再写一份。内网部署需要一份 blob volume：同 data owner 授权范围内秒传/秒下，同时保持旧 URI 与加密信封。

根设计：`openspec/changes/drivebridge-intranet-cas-mount-v1/`。本包只负责 Server 元数据、HTTP/SDK/MCP 与备份清单。

## What Changes

- 新增 Blob 记录：明文 SHA-256、逻辑 size、引用计数、object_key、加密空间 id（可空）。
- create_upload 在授权命中时直接 completed，`bytes_transferred=0`，新 version 指向已有 blob。
- Complete 写入 blob 键；并发同一 sha256 后到者改引用并删除重复对象。
- 备份清单增加 blob 交叉；`blob stat` / `blob gc --dry-run` 为 owner CLI。
- HTTP/MCP/SDK 增量可选字段 `blob_reused`；不改旧路径、旧必填项、旧错误码。

本包不实现 WebDAV/FUSE，不读取客户端本机路径，不跨实例联邦。

## Capabilities

### New Capabilities
- `drivebridge-cas-blob-store`：实例内 blob 身份、引用计数、秒传/秒下。

### Modified Capabilities
- `drivebridge-personal-service`：上传完成可复用 blob；对象键双写。

## Impact

Owner：`backend-server/drivebridge`。涉及 `internal/service`、`sdk`、CLI、备份清单与 `docs/`。GORM 只加表/可空列。保持 `CGO_ENABLED=0`。
