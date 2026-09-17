# 本机 CAS 缓存与挂载

根合同：仓库根归档 [`openspec/changes/archive/2026-09-17-drivebridge-intranet-cas-mount-v1/`](../../../openspec/changes/archive/2026-09-17-drivebridge-intranet-cas-mount-v1/)（含 `vfs-api.md`）。实现包 [`openspec/changes/drivebridge-local-cas-mount-v1/`](../openspec/changes/archive/2026-09-17-drivebridge-local-cas-mount-v1/)。

同空间 `ReuseUpload` 在后端支持 copy 时零字节上传；本机新增跨文件下载缓存、远程 Server 秒传消费（`blob_reused` 命中时本机不发分片），以及把授权空间挂到本机的只读 WebDAV。

```bash
drivebridge upload --space creative --file ref.png
drivebridge download --ref 'drivebridge://file/FILE_REF' --out refs/ref.png
drivebridge mount webdav --space creative --listen 127.0.0.1:19080
drivebridge mount status
drivebridge mount stop
drivebridge mount fuse   # 默认二进制返回 unsupported，请用 webdav
```

## CAS 缓存

下载先比目标路径和 `<local_root>/.drivebridge-cas/sha256/<aa>/<bb>/<sha256>`（文件 0600、目录 0700，临时文件 + 硬链接发布）。摘要与固定 version 一致则零字节完成（回执 `cache_hit=true`、`bytes_transferred=0`）；缓存命中前仍会 stat 远端校验 version，version 变化绝不发布旧字节。CAS 目录不出现在 `ls`/`search` 用户树和挂载树。

## 挂载

挂载默认只读、回环、用户终端启动；会话 token 只写进 state 目录的私有 launch 文件（`mount-webdav.url`，0600），不打印到 stdout。Agent 不能启动挂载；MCP 最多 `mount_status`（listen、mode、space_ids、alive；alive 是真实回环拨测）。WebDAV 拒绝 Agent token（`db_` 前缀）。

方法映射：PROPFIND→Lookup/Readdir、GET/HEAD→Open（冻结 version，支持 Range）、MKCOL→Mkdir、PUT→Create（仅 `--writable`，整对象发布后原 upload 合同生效）、MOVE→Rename（同空间）、DELETE→trash、LOCK→`unsupported`。setattr/chmod 等 POSIX 幻觉一律拒绝。非回环 listen 必须 `--lan`，否则 `listen_not_allowed`；同一 state 只允许一个挂载，冲突 `mount_already_running`。这不是 `drivebridge sync`。

能力资源新增 `cas_reuse` 与 `mount`（`start=cli_only`、`fuse=unsupported_default_build`）。

## 回退与回滚

需要内核 FUSE 时：默认构建不含，改用可选构建或把 `rclone mount` 指到 WebDAV URL。回滚：停 WebDAV 进程即卸载视图，CAS 目录可留；关闭远程秒传后 upload 走原分片路径。

默认构建保持 `CGO_ENABLED=0`，libfuse 不进默认 CLI。Server 侧 blob 表归 `backend-server/drivebridge`，本包不拥有；无公网挂载能力。
