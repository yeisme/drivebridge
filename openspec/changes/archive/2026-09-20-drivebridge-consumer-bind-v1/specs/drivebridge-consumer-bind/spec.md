## ADDED Requirements

### Requirement: 本机绑定记录由 CLI 写入

DriveBridge SHALL 提供 `drivebridge bind add|list|show|revoke`。绑定记录 SHALL 由 CLI 或 application service 写入，包含 consumer、project_ref、space、prefix、purpose、可选 connection_id 与幂等键。同一幂等键且同一字段 SHALL 复用成功；同一幂等键且不同字段 SHALL 拒绝并返回 `bind_mismatch`。调用方自称的 consumer 或 project 标签 SHALL NOT 构成认证。SHALL NOT 打印凭据、密钥文件或签名 URL。

#### Scenario: 创建绑定
- **WHEN** `drivebridge bind add --consumer pinax --project drama-1 --space materials --prefix projects/drama-1 --purpose materials`
- **THEN** 成功信封含 consumer、project_ref、space、prefix、purpose，且不出现 token 或签名 URL

#### Scenario: 同键同字段复用
- **WHEN** 以同一 `--idempotency-key` 再次提交相同 consumer/project/space/prefix/purpose
- **THEN** 成功复用既有记录

#### Scenario: 同键不同字段拒绝
- **WHEN** 同一幂等键声明了不同的 prefix 或 purpose
- **THEN** 返回 `bind_mismatch`，既有记录不变

#### Scenario: 未知 purpose 拒绝
- **WHEN** `--purpose` 不是 `vault`、`notes-attachments`、`materials`、`saveback`、`owner-store`
- **THEN** 返回 `bind_purpose_unknown`

### Requirement: 默认绑定发现只含 bind_default kinds 与已 adopt 空间

`drivebridge bind discover` 在未传 `--allow-generic` 时 SHALL NOT 列出逃生舱 kinds（`openlist`、`webdav`、`smb`、`sftp`、`alias`）。默认发现 SHALL 包含 kind 为 bind_default（`local`/`s3`/`onedrive`/`gdrive`）的空间，以及已有 adopt 记录的空间。

#### Scenario: 默认发现不含逃生舱
- **WHEN** 配置了 webdav/openlist 等逃生舱空间并运行 `bind discover`（无 `--allow-generic`）
- **THEN** 结果不含这些空间，仍可含 local/s3/onedrive/gdrive 与已 adopt 空间

#### Scenario: 已 adopt 空间可发现
- **WHEN** 某 local 或 s3 空间已 `storage adopt`
- **THEN** `bind discover` 列出该空间且标记 adopted

#### Scenario: 显式允许 generic
- **WHEN** `bind discover --allow-generic`
- **THEN** 逃生舱空间可出现在结果中

### Requirement: 逃生舱空间默认不能绑定

绑定目标为逃生舱空间且未传 `--allow-generic` 时 SHALL 失败，并提示改用已 adopt 的 local/S3 或 OneDrive/Google Drive。

#### Scenario: 拒绝绑定 OpenList/NAS
- **WHEN** `bind add` 的 `--space` 是 openlist/webdav/smb/sftp/alias 且无 `--allow-generic`
- **THEN** 返回稳定错误（如 `bind_escape_denied`），提示改用已 adopt 的 local/S3 或 OneDrive/GDrive

#### Scenario: 可绑定 adopt 来的项目存储
- **WHEN** Pinax vault 目录已被 adopt
- **THEN** 该空间可按 purpose=`owner-store` 或 `vault`/`materials` 绑定

### Requirement: 绑定范围前缀围栏

存在绑定后，针对该 bind 或传入 consumer+project 的操作 SHALL 拒绝前缀外路径，至少覆盖 list、stat、upload、delete/trash、mkdir，并返回 `bind_prefix_denied`。未带绑定范围的既有 relay/upload/space 命令 SHALL 保持原义。

#### Scenario: 越界前缀拒绝
- **WHEN** 已绑定前缀为 `projects/drama-1/` 的身份请求 `projects/drama-2/ref.png` 的 list/stat/upload/mkdir/trash
- **THEN** 返回 `bind_prefix_denied`，不返回跨前缀字节或引用

#### Scenario: 前缀内允许
- **WHEN** 同一身份请求 `projects/drama-1/ref.png` 或列举 `projects/drama-1`
- **THEN** 操作不被 `bind_prefix_denied` 拒绝

#### Scenario: 未绑定范围保持兼容
- **WHEN** 不传 `--consumer`/`--project`/`--bind` 而 `ls`/`upload` 已绑定空间
- **THEN** 行为与 bind 落地前一致

### Requirement: 撤销停止该 bind 的新读取

`drivebridge bind revoke <bind-id>` SHALL 阻止该 bind 后续的 list/stat/import。SHALL NOT 声称收回已落地的 owner 副本，SHALL NOT 删除空间或 adopt 记录。

#### Scenario: 撤销后新读取
- **WHEN** 撤销某 bind 后再以该 bind 或同一 consumer+project 做 list/stat
- **THEN** 返回 `bind_revoked`（或等价稳定码）；未带绑定范围的 list 仍可用；adopt 记录仍在

### Requirement: MCP 加法只读发现

MCP capabilities SHALL 可广告 `bind_discover` 与 `bind_list`。对应工具 SHALL 只读，SHALL NOT 接受任意远端 URL 或凭据。调用方标签仍不是认证。

#### Scenario: capabilities 广告 bind 发现
- **WHEN** 读取 capabilities
- **THEN** 可发现 `bind_discover` 与 `bind_list`，且不出现凭据字段

#### Scenario: bind_discover 默认过滤逃生舱
- **WHEN** 调用 MCP `bind_discover` 且未设 allow_generic
- **THEN** 结果与 CLI 默认 discover 相同，不含逃生舱空间
