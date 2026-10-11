# drivebridge-cas-blob-store Specification

## Purpose
TBD - created by archiving change drivebridge-cas-blob-store-v1. Update Purpose after archive.
## Requirements
### Requirement: Blob 记录与引用计数

系统 SHALL 为每个明文 SHA-256（加密空间叠加 space_id）保存 blob 记录、逻辑大小、引用计数与 object_key。version 发布增加计数，purge 且无剩余 version 减少计数。计数到 0 SHALL NOT 自动删除对象。

#### Scenario: 两路径共享
- **WHEN** 同一用户两个 version 具有相同明文 SHA-256 且同加密状态
- **THEN** 只有一份对象，引用计数为 2

#### Scenario: 回收默认 dry-run
- **WHEN** 操作者运行 blob gc 未带确认
- **THEN** 只报告可回收项，不删除对象

### Requirement: 秒传不绕过授权与幂等

create_upload 命中 SHALL 仍校验路径可写、配额与幂等指纹。命中结果 SHALL 创建可查询的 completed operation。

#### Scenario: 秒传占用配额
- **WHEN** 秒传成功
- **THEN** `used_bytes` 增加逻辑 size，`reserved_bytes` 不残留该 size

#### Scenario: 对象缺失
- **WHEN** 元数据有 blob 但存储读回失败
- **THEN** 返回 `blob_unavailable`，不发布空文件，不把命中当作成功

### Requirement: 秒传范围是 data owner

blob 命中 SHALL 使用 `Principal.dataOwner()`。团队空间内可写成员 MAY 共享 blob。团队外或另一实例 SHALL 视为未命中。

#### Scenario: 团队成员秒传
- **WHEN** 同一团队 data owner 下第二名成员上传已有 SHA-256
- **THEN** 零字节完成，引用计数增加

#### Scenario: GET blob 越权
- **WHEN** 无读权的主体请求 `GET /api/v1/blobs/{sha256}`
- **THEN** 返回 `not_found`，不返回 ref_count

### Requirement: 无挂载 HTTP

本服务 SHALL NOT 新增启动本机挂载的 HTTP 路径。Range 读取 SHALL 继续走 `GET /api/v1/files/{id}/content`。

#### Scenario: 内容 Range
- **WHEN** 授权客户端对固定 version 发送 Range
- **THEN** 返回 206 与对应字节，不暴露 blob 键

