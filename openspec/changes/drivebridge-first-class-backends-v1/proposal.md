## Why

根包 `drivebridge-consumer-binding-v1` 把产品面收成两层一等：各项目已有的 `local`/`s3` 是可管理默认存储，`onedrive`/`gdrive` 是个人网盘。当前 `storage kinds` 把除 OneDrive 外的全部 kind 当成同一 generic 目录，逃生舱（OpenList/NAS 等）与默认存储混在一起，后续 bind/adopt 发现无法诚实过滤。

本 change 只把目录标清楚，不实现 adopt、不晋升 gdrive MD5、不删除 generic 后端。

## What Changes

- `KindDef` 加法 JSON 字段 `tier`、`bind_default`；既有 `kind`/`display`/`backend`/`requires`/`notes`/`capabilities` 保留。
- 项目默认存储：`local`、`s3` → `tier=first_class_store`，`bind_default=true`。`local` 是零拷贝目录视图，不是新网盘厂商。
- 个人网盘：`onedrive`、`gdrive` → `tier=first_class_cloud`，`bind_default=true`。
- 逃生舱：`openlist`、`webdav`、`smb`、`sftp`、`alias` → `tier=escape`，`bind_default=false`。
- 默认 bind/adopt 发现（`bind_default_kinds`）只含 `bind_default=true` 的 kinds；完整 `storage_kinds` 仍列出逃生舱。
- generic 能力真值不变：`transfer=true`，`direct_url=false`，`reference_handoff=false`，不设 `server_side_copy` 键。OneDrive 仍是唯一直链 + 参考交接后端。

## Capabilities

### New Capabilities
- `drivebridge-first-class-backends`：两层一等 kind 目录与默认 bind 发现子集。

## Impact

Owner：`mcp/drivebridge`。触及 catalog、capabilities 加法键、`storage kinds` 投影与 recipe 文档。不改 `storage add`/`probe` 行为，不实现 adopt/bind 命令，不改 rclone 后端。
