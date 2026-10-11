## Why

现行备份只复制元数据与实例 ID（`backup` 命令），对象字节备份"由存储操作者管理"、恢复正确性没有可执行的校验。个人单管理员场景下（双人门已砍），备份必须是"一个人也能确信可恢复"的：有清单、能校验、有演练手册。

## What Changes（设计，实现归后续波）

- 对象清单 manifest：备份时导出 版本→对象 key→明文摘要 映射，与元数据备份同目录。
- `backup --verify`：清单与元数据交叉核对 + 抽样对象存在性探测（只读），接 doctor 报告面。
- 恢复演练 runbook：deployment.md §5 增强为可勾选步骤；密钥托管指引写明单人秘密渠道。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增 Backup verification and object manifest 需求。

## Impact

不改备份目录结构既有字段（向后兼容追加 manifest.json）；不动 S3 字节复制策略（对象备份仍归存储层，清单只提供校验依据）。加密备份包与 content-encryption-v1 联动（信封加密落地后备份天然密文安全）。
