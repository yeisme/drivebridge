## ADDED Requirements

### Requirement: 存储 kind 两层一等

DriveBridge SHALL 在 kind 目录中为每个 kind 标注 `tier` 与 `bind_default`，分成两层一等而不是「只剩三个网盘」：

- 项目默认存储（可管理）：`local`、`s3` SHALL 使用 `tier=first_class_store` 且 `bind_default=true`
- 个人网盘：`onedrive`、`gdrive` SHALL 使用 `tier=first_class_cloud` 且 `bind_default=true`
- 逃生舱：`openlist`、`webdav`、`smb`、`sftp`、`alias` SHALL 使用 `tier=escape` 且 `bind_default=false`

`local` SHALL 是零拷贝目录视图，SHALL NOT 被宣传为新的网盘提供商。`storage kinds` 与 `capabilities.storage_kinds` SHALL 继续列出全部 kinds，包括逃生舱。SHALL NOT 删除 generic 后端。

#### Scenario: 目录分层标签
- **WHEN** 调用 `drivebridge storage kinds` 或读取 `capabilities.storage_kinds`
- **THEN** `local` 与 `s3` 的 `tier` 为 `first_class_store` 且 `bind_default` 为 true；`onedrive` 与 `gdrive` 的 `tier` 为 `first_class_cloud` 且 `bind_default` 为 true；`openlist`/`webdav`/`smb`/`sftp`/`alias` 的 `tier` 为 `escape` 且 `bind_default` 为 false

#### Scenario: local 不是新网盘
- **WHEN** 读取 local kind 的 display/notes 或 recipe 文档
- **THEN** 将其描述为可管理默认存储 / 零拷贝目录视图，而不是云厂商或个人网盘

#### Scenario: 逃生舱仍在完整目录
- **WHEN** 列出完整 storage kinds
- **THEN** 结果仍包含 `openlist`、`webdav`、`smb`、`sftp`、`alias`，且这些 kind 仍可按既有 `storage add --kind` 合同接入

### Requirement: 默认绑定发现只含 bind_default kinds

默认 bind/adopt 发现面 SHALL 只包含 `bind_default=true` 的 kinds。该子集 SHALL 通过加法字段 `bind_default_kinds` 出现在 `storage kinds` 信封与 MCP `capabilities` 中。逃生舱 SHALL NOT 出现在该默认发现面。本要求 SHALL NOT 实现 `storage adopt` 或 consumer bind 命令。

#### Scenario: 默认发现不含逃生舱
- **WHEN** 消费者读取 `bind_default_kinds` 且未显式请求完整目录
- **THEN** 结果只含 `local`、`s3`、`onedrive`、`gdrive`，不含 `openlist`/`webdav`/`smb`/`sftp`/`alias`

#### Scenario: 完整目录与默认发现并存
- **WHEN** 同一 `storage kinds` 或 `capabilities` 响应被读取
- **THEN** `kinds`/`storage_kinds` 含逃生舱，`bind_default_kinds` 不含逃生舱

### Requirement: 一等发现不宣称特权能力

local、s3、gdrive 即使进入默认发现，SHALL 保持 generic 后端与既有能力真值：`transfer=true`，`direct_url=false`，`reference_handoff=false`，且 SHALL NOT 设置 `server_side_copy` 键。OneDrive SHALL 仍是唯一同时提供 `direct_url` 与 `reference_handoff` 的 kind。既有 `KindDef` JSON 字段 SHALL 保留。

#### Scenario: 非 OneDrive 一等 kind 仍为 generic
- **WHEN** 检查 `local`、`s3`、`gdrive` 的 catalog 条目
- **THEN** `backend` 为 `generic`，capabilities 为 transfer-only，且没有 `server_side_copy` 键

#### Scenario: OneDrive 保持特权真值
- **WHEN** 检查 `onedrive` catalog 条目
- **THEN** `direct_url` 与 `reference_handoff` 为 true，且 `tier=first_class_cloud`、`bind_default=true`

#### Scenario: 既有 JSON 字段仍在
- **WHEN** 序列化任意 kind
- **THEN** 响应含 `kind`、`display`、`backend`、`requires`、`capabilities`，并加法可选的 `tier` 与 `bind_default`
