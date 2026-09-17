## Why

本机适配器已有同空间 `ReuseUpload` 与 `link`，但下载仍可能重复拉网，也没有把授权空间树挂到本机文件系统。用户要 hash 去重下载、HTTP/MCP/CLI 继续可用，以及类似 FUSE 的挂载。默认二进制必须保持 `CGO_ENABLED=0`。

根设计：`openspec/changes/drivebridge-intranet-cas-mount-v1/`。本包负责本机 CAS 缓存、远程秒传消费、VFS/WebDAV 与可选 FUSE 回退。

## What Changes

- 下载前查目标路径与 `.drivebridge-cas`；摘要一致则零字节。
- 远程 upload 把命中交给 Server；本机 `ReuseUpload` 保持同空间 copy 优化。
- `drivebridge mount webdav` 默认回环只读；`mount fuse` 在默认构建返回 `unsupported`。
- MCP 增加只读 `mount_status`；Agent 不能启动挂载。
- 能力资源增加 `cas_reuse` 与 `mount`。

不拥有 Server blob 表，不改变 OneDrive 直链交接，不把 sync 当挂载。

## Capabilities

### New Capabilities
- `drivebridge-mount-surface`：VFS/WebDAV 与可选 FUSE 边界。

### Modified Capabilities
- `drivebridge-local-files`：本机 CAS 缓存与下载去重。

## Impact

Owner：`mcp/drivebridge`。涉及 `internal/bridge`、`cmd/drivebridge`、docs。可选 fuse 二进制不得成为默认构建依赖。
