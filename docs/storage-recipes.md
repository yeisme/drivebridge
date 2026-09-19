# 存储接入 recipe 卡

先看目录与探测，再选一张卡执行；所有命令都在用户本人终端运行。

```bash
drivebridge storage kinds          # 能力目录：每类的必填旗标、tier 与能力真值
drivebridge storage probe --remote <remote> [--remote-path <path>]
drivebridge storage add --kind <kind> …
drivebridge storage adopt --kind local|s3 --consumer <id> --space <slug> …
drivebridge storage adopt status
drivebridge ls --space <space>     # 添加后冒烟
```

目录是两层一等，不是「只剩三个网盘」：

| 层 | kinds | `tier` | `bind_default` |
| --- | --- | --- | --- |
| 项目默认存储（可管理） | `local`、`s3` | `first_class_store` | true |
| 个人网盘 | `onedrive`、`gdrive` | `first_class_cloud` | true |
| 逃生舱 | `openlist`、`webdav`、`smb`、`sftp`、`alias` | `escape` | false |

`local` 是零拷贝目录视图，用来托管各项目已有的工作副本 / vault / output-root，**不是新的网盘厂商**。`storage kinds` 仍列出全部 kinds（含逃生舱）。默认 bind/adopt 发现只使用信封里的 `bind_default_kinds`（`local`/`s3`/`onedrive`/`gdrive`）；逃生舱仍可 `storage add --kind`，只是不进入默认产品发现面。

项目绑定（本机 consumer bind）把 consumer+project 限制在某个空间的允许前缀与 purpose 上。`--consumer` / `--project` 是调用方自称的标签，不是认证：

```bash
drivebridge bind discover                 # 默认：bind_default kinds + 已 adopt 空间
drivebridge bind discover --allow-generic # 才列出 openlist/webdav/smb/sftp/alias
drivebridge bind add --consumer pinax --project drama-1 \
  --space pinax-vault --prefix notes --purpose vault
drivebridge bind list
drivebridge bind show <bind-id>
drivebridge bind revoke <bind-id>         # 停该 bind 的新读取；不删空间/adopt/owner 文件
```

带 `--consumer` 与 `--project`（或 `--bind`）的 `ls`/`stat`/`upload`/`mkdir`/`trash` 对前缀外路径返回 `bind_prefix_denied`。不带这些旗标的既有命令保持原义。purpose 首批：`vault`、`notes-attachments`、`materials`、`saveback`、`owner-store`。绑定逃生舱空间需 `--allow-generic`，否则提示改用已 adopt 的 local/S3 或 OneDrive/Google Drive。

约定：`storage add` 的 `--max-bytes` 缺省 1073741824；`--remote-path` 对 gdrive/webdav/smb/sftp/alias/openlist 可空（= 远端根），s3 必填（首段为桶名）、local 是绝对目录；探测只读且**列举整个远端根**（大根目录可能耗时），写权限在首次上传才验证（回执 `roundtrip_status=not_verified`）；除 OneDrive 外全部 transfer-only（`access_unsupported`/`handoff_not_available`），一等发现标签不改变这条能力真值。webdav 密码经 rclone obscure 入库——可逆混淆而非加密，与 rclone 生态一致，配置文件本身须 0600。

## 本地目录（可管理默认存储，不是网盘）

```bash
drivebridge storage add --kind local --space docs \
  --remote-path /absolute/source-dir \
  --local-root /absolute/workspace
```

自动创建 alias remote `db-local-<space>`（目录是零复制视图）。`tier=first_class_store`，进入默认 bind/adopt 发现；用来托管 owner 已有本地根，不把 DriveBridge 宣传成新的云厂商。不写远端即可与挂载、同步组合。

要把 **owner 已经在用的** 同一目录交给 DriveBridge 浏览（不复制、不改写 `.pinax` 等元数据），用 adopt 而不是再 `storage add` 一份：

```bash
drivebridge storage adopt --kind local --consumer pinax --space pinax-vault \
  --root /absolute/vault \
  [--local-root /absolute/parent-or-same] [--idempotency-key k]
```

`--local-root` 缺省等于 `--root`，且必须是该目录或其父目录。成功信封含 `copied=false`。同一幂等键换位置会 `adopt_mismatch`。已 adopt 空间写入 `.pinax` / `.scaena` 元数据 / sqlite / token 返回 `owner_protected`；list/stat 仍允许。CAS 对象字节（`.scaena/objects/sha256/**`）可 transfer。

## S3 兼容（可管理默认存储；MinIO/COS/OSS/R2/B2）

前置：rclone `type = s3` remote（MinIO 例：`rclone config create minio s3 provider MinIO endpoint http://nas:9000 access_key_id … secret_access_key …`）。

```bash
drivebridge storage add --kind s3 --space blobs \
  --remote minio --remote-path '<bucket>/<prefix>' \
  --local-root /absolute/path
```

`--remote-path` 第一段必须是桶名。`tier=first_class_store`，进入默认 bind/adopt 发现。想要实例级去重 + 信封加密，请改走 `drivebridge-server platform-connect`（平台卷角色），两者不混用。

挂接 **同一** 已有桶前缀（不复制对象）：

```bash
drivebridge storage adopt --kind s3 --consumer eikona --space eikona-exports \
  --remote minio --remote-path '<bucket>/<prefix>' \
  --local-root /absolute/path [--idempotency-key k]
```

DriveBridge 不解密 Capsa 密文。doctor **不自动识别** 密文前缀，对 s3 adopt 报 `opaque_encrypted not auto-detected`；把该前缀当不透明对象即可。平台 CAS 卷不要与项目前缀混 adopt。

## OneDrive（个人网盘）

不走 `storage add`：`drivebridge auth --auto` → `drivebridge setup`。唯一具备直链与 Scaena 参考交接的后端。`tier=first_class_cloud`，进入默认 bind/adopt 发现。

## Google Drive（gdrive）

前置：rclone 已有 `type = drive` 的 remote（可用 `rclone config create mygd drive` 完成 OAuth）。

```bash
drivebridge storage add --kind gdrive --space g \
  --remote mygd --remote-path '<drive 内子目录，可空>' \
  --local-root /absolute/path
```

版本钉住当前为 `大小+mtime`；原生 MD5 复用是后续晋升项。Google Docs 在线文档不支持。`tier=first_class_cloud`，进入默认 bind/adopt 发现，但仍是 transfer-only（无直链、无参考交接）。

## OpenList（逃生舱：百度/阿里/115/夸克等聚合）

前置：自托管 OpenList（钉住已验收版本、专用受限账号、建议内网/Tailscale 暴露）。

```bash
drivebridge storage add --kind openlist --space cloud \
  --url 'http://<内网地址>:5244/dav' \
  --webdav-user '<专用账号>' \
  --webdav-pass-file /absolute/private/pass.0600 \
  --remote-path '<OpenList 内路径>' \
  --local-root /absolute/path
```

`storage probe` 会顺带探测源站 `/api/public/settings`：识别为 OpenList 时结果带 `aggregator/openlist` 与版本号（识别失败不影响 WebDAV 可用性判定）：

```bash
drivebridge storage probe --remote db-openlist-cloud --kind openlist --json   # "aggregator":"openlist","aggregator_version":"…"
```

自动创建 webdav remote（vendor=other，密码 obscure 入库，绝不回显）。同名 remote 已存在且 url/用户一致则复用，否则 `remote_exists`。pass-file 必须 0600。国内网盘元数据抖动 → `file_version_changed` 属诚实行为。`tier=escape`，不进入默认 bind/adopt 发现。

## SMB / SFTP / WebDAV / alias（逃生舱：NAS）

前置：对应类型的 rclone remote。

```bash
drivebridge storage add --kind smb --space nas \
  --remote mynas --remote-path '<共享内路径>' \
  --local-root /absolute/path
```

`--kind` 支持 s3/smb/sftp/webdav/alias/gdrive/openlist/local；kind 只决定引导与自动 remote，空间后端（除 OneDrive）统一是 transfer-only generic。`smb`/`sftp`/`webdav`/`alias` 为逃生舱，`tier=escape`，不进入默认 bind/adopt 发现。
