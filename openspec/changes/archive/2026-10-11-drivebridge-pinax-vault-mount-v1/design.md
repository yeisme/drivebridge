# Design

## Context

2026-10-01：旧mount/overlay继续兼容，不自动迁移。新 [Electron阅读受管绑定](../drivebridge-desktop-knowledge-reader-v1/design.md) 是另一个显式opt-in：本包 `writable=true` 只表示文件通道，不能充当owner guard证据；新绑定须证明content roots/别名/在途写都受控，无法证明就拒绝。此说明不改旧receipt或自动停挂载。

根合同已归档于仓库根 `openspec/changes/archive/2026-09-21-pinax-drivebridge-mounted-vault-v1/`。WebDAV 挂载交付端口；Pinax 要的是目录。SQLite 不能放在 VFS 上。

## Decisions

### D1 本地 alias 用符号链接叠加，不启 FUSE

`kind=local` 空间的内容根是 rclone alias 目标目录。`--vault-root` 是另一个本机目录：

- 为内容树顶层项（默认至少 `notes/`）建立指向内容根的符号链接。
- `.pinax`、`.git`、`.obsidian`、忽略文件留在 vault 根，从不链接、从不上传。
- Pinax 经符号链接写 Markdown，真实字节落在空间内容根；DriveBridge 仍看空间目录，不跟踪 vault 根上的链接。

S3（rclone `type=s3`）：exec 本机 `rclone mount remote:bucket/prefix` 到 `state/mnt/<space>`，`--vfs-cache-mode writes`、`--daemon`，默认二进制不链 libfuse。挂载成功后再走同一套符号链接叠加。rclone 不在 PATH、FUSE 不可用或 mount 失败 → `mount_path_adapter_missing`。不把 `mount webdav` 端口写成成功的 pinax-vault。`mount stop` 先拆叠加再 `rclone umount`。

### D2 预设不调用 Pinax

`preset pinax-vault` 要求空间已存在（先 `storage add`/`adopt`）。成功信封含 `vault_root`、`writable=true`、`preset=pinax-vault` 以及人读下一步：`pinax init` / `storage set local`。

### D3 生命周期

叠加不是阻塞 HTTP 服务。`RegisterMount` 仍占同一 state 单槽。`mount stop` 删除内容符号链接与挂载回执，保留本机 `.pinax` 与内容根对象。`mount status` 的 `alive`：vault 根存在且内容链接可解析。

### D4 Capsa 前缀拒绝

内容根出现 Capsa `protocol.json` 布局则 `preset_prefix_is_capsa`。

## 验证

```bash
go test ./internal/bridge ./cmd/drivebridge -run 'Overlay|PinaxVault|Mount' -count=1
CGO_ENABLED=0 go build ./cmd/drivebridge
```
