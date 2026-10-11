# 内网 CAS 与 blob 复用

根合同：仓库 `openspec/changes/archive/2026-09-17-drivebridge-intranet-cas-mount-v1/`。本文件只说明 Server 侧；实现包 `openspec/changes/drivebridge-cas-blob-store-v1/` 已落地代码（本包不实现 WebDAV/FUSE 挂载，挂载归 cli/drivebridge owner）。

## 现在 vs 目标

每个 version 曾独占 `instance/owner/space/version` 对象。现在：明文 SHA-256 是 blob 键，file/version URI 不变，同 data owner 授权范围内二次上传/下载不写、不拉重复字节。旧 version 继续按自身 `object_key` 读。

运维命令：

```bash
drivebridge-server blob stat <sha256>
drivebridge-server blob gc            # 默认 dry-run，只报告可回收项
drivebridge-server blob gc --yes      # 确认后才删除 refs=0 的对象与记录
```

`create_upload` 仍要 `space_id`、`path`、`size_bytes`、小写 `sha256` 与 `Idempotency-Key`。命中时 operation 直接 `completed`，`bytes_transferred=0`，成功体带可选 `blob_reused=true`；命中前会探测对象存在，元数据有 blob 但存储读回失败报 `blob_unavailable`（503），不发布空文件。未命中保持分片。越权 `GET /api/v1/blobs/{sha256}` 返回 `not_found`，不确认摘要是否存在。

对象键：明文 `<instance>/blobs/<sha256>`，加密 `<instance>/blobs/<space>/<sha256>`。加密空间同空间可共享密文 blob（对象头自带 nonce，可自描述解密）；跨空间或明文/加密之间必须重写。直链与 `prepare_reference` 规则不变。

备份清单在 `versions` 之外记录 `blob_sha256` 与 `counts.blobs` 交叉，恢复校验包括 blob 引用计数对账；旧 `object_key` 字段保留，无 blob 列的旧备份照常生成清单。`blob gc` 默认 dry-run，引用计数为 0 也不自动删。

秒传范围是 `Principal.dataOwner()`：个人空间按用户，团队空间按团队；团队内可写成员可命中。外人同摘要仍当未命中。文件字节继续走 `GET /api/v1/files/{id}/content`，Range 读取走既有 `/files/{id}/media` 通道（206），均不暴露 blob 键。没有挂载 HTTP。

并发：同一 sha256 的 Complete 先写确定性 blob 键，再在发布事务中占 blob 行——先到建行、后到加引用，只余一份对象。purge 只减引用。

本轮规格不等于部署可用。真实 COS/内网磁盘验收另报。
