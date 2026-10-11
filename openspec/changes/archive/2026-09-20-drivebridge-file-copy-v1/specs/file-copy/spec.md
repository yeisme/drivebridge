## ADDED Requirements

### Requirement: 受管文件复制
系统 SHALL 提供 `POST /api/v1/files/{id}/copy`，将固定源版本复制为新文件身份。请求包含 `target_space_id`、`target_path`、`expected_version` 与可选 `conflict`（`rename` 默认或 `fail`）。`Idempotency-Key` SHALL 必填。目标 SHALL 为受管 S3 空间。SHALL NOT 覆盖已有路径，SHALL NOT 把该操作记为 archive。

#### Scenario: 同空间秒传
- **WHEN** 同一 data owner 受管目标已有相同 SHA-256 blob
- **THEN** 操作立即 completed，`blob_reused` 为 true，不写新对象；新旧文件均可按原 URI 读取相同字节

#### Scenario: 冲突改名
- **WHEN** `conflict` 为 rename 或省略，且目标路径已被占用
- **THEN** 使用 `name (2).ext` 递增路径创建副本，源文件保持不变

#### Scenario: 冲突失败
- **WHEN** `conflict` 为 fail 且目标路径已被占用
- **THEN** 返回 `name_conflict` 且不创建文件

#### Scenario: 源版本变化
- **WHEN** `expected_version` 与当前头版本不同，或源在回收站，或源有进行中的传输
- **THEN** 分别返回 `version_conflict`、`file_trashed` 或 `file_in_use`

#### Scenario: 非受管目标
- **WHEN** 目标空间连接缺失、禁用或非 s3
- **THEN** 在任何字节传输或配额预留之前返回 `unsupported`

#### Scenario: 幂等重放
- **WHEN** 使用同一 Idempotency-Key 重放相同复制请求
- **THEN** 返回原操作，不创建第二份文件
