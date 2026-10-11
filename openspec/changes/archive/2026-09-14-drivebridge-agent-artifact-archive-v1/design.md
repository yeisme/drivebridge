# Design: drivebridge-agent-artifact-archive-v1

## 决策

1. **归档是副本不是引用**：`link` 已覆盖零拷贝引用；归档目标语义是"用户长期持有的受管副本"，因此走服务端流 `Download`（含加密空间解密）→ 分片 `putPart` → `Complete`，端到端复用上传 SHA256 校验，不经客户端。
2. **冲突安全路径**：目标路径已存在（未回收）时按 `name (2).ext` 递增改名，改名后重跑 `allow(write)`，保证改名不会绕过目录级授权；绝不覆盖既有文件。
3. **幂等与恢复**：归档请求体单独指纹（`archive:` 前缀 + JSON），同一 Idempotency-Key 重放返回既有操作；归档 kind 记录 `ArchiveFileID`/`ArchiveVersionID`/`ArchiveHeadVersion`，`resume` 按记录参数重新传输，永不生成内容。中断窗口内 `ArchiveHeadVersion` 与源当前头不一致即 `version_conflict`，防止"恢复"悄悄归档了被覆盖后的新内容。
4. **收件箱不加新表**：复用 `Operation` 表，30 天窗口 + 上限 200；权限过滤在 limit 截断之后执行（结果可少于请求数量），避免按身份建二级索引；数据库失败如实返回 `database_unavailable` 而非空列表冒充成功。
5. **并发**：全局 `archiveMu` 互斥冲突改名检查与写入窗口；归档量级低（用户手工挑选产物），不做 per-path 锁。
6. **兼容**：`Operation` JSON 仅增 omitempty 字段，SDK 增量方法，既有上传/下载/操作契约不变。
