# 存储接入 recipe 卡

先看目录与探测，再选一张卡执行；所有命令都在用户本人终端运行。

```bash
drivebridge storage kinds          # 能力目录：每类的必填旗标与能力真值
drivebridge storage probe --remote <remote> [--remote-path <path>]
drivebridge storage add --kind <kind> …
drivebridge ls --space <space>     # 添加后冒烟
```

约定：`storage add` 的 `--max-bytes` 缺省 1073741824；`--remote-path` 对 gdrive/webdav/smb/sftp/alias/openlist 可空（= 远端根），s3 必填（首段为桶名）、local 是绝对目录；探测只读且**列举整个远端根**（大根目录可能耗时），写权限在首次上传才验证（回执 `roundtrip_status=not_verified`）；除 OneDrive 外全部 transfer-only（`access_unsupported`/`handoff_not_available`）。webdav 密码经 rclone obscure 入库——可逆混淆而非加密，与 rclone 生态一致，配置文件本身须 0600。

## Google Drive（gdrive）

前置：rclone 已有 `type = drive` 的 remote（可用 `rclone config create mygd drive` 完成 OAuth）。

```bash
drivebridge storage add --kind gdrive --space g \
  --remote mygd --remote-path '<drive 内子目录，可空>' \
  --local-root /absolute/path
```

版本钉住当前为 `大小+mtime`；原生 MD5 复用是后续晋升项。Google Docs 在线文档不支持。

## OpenList（百度/阿里/115/夸克等聚合）

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

自动创建 webdav remote（vendor=other，密码 obscure 入库，绝不回显）。同名 remote 已存在且 url/用户一致则复用，否则 `remote_exists`。pass-file 必须 0600。国内网盘元数据抖动 → `file_version_changed` 属诚实行为。

## S3 兼容（MinIO/COS/OSS/R2/B2）

前置：rclone `type = s3` remote（MinIO 例：`rclone config create minio s3 provider MinIO endpoint http://nas:9000 access_key_id … secret_access_key …`）。

```bash
drivebridge storage add --kind s3 --space blobs \
  --remote minio --remote-path '<bucket>/<prefix>' \
  --local-root /absolute/path
```

`--remote-path` 第一段必须是桶名。想要实例级去重 + 信封加密，请改走 `drivebridge-server platform-connect`（平台卷角色），两者不混用。

## SMB / SFTP / WebDAV（NAS）

前置：对应类型的 rclone remote。

```bash
drivebridge storage add --kind smb --space nas \
  --remote mynas --remote-path '<共享内路径>' \
  --local-root /absolute/path
```

`--kind` 支持 s3/smb/sftp/webdav/alias/gdrive/openlist/local；kind 只决定引导与自动 remote，空间后端统一是 transfer-only generic。

## 本地目录 / alias 零复制视图

```bash
drivebridge storage add --kind local --space docs \
  --remote-path /absolute/source-dir \
  --local-root /absolute/workspace
```

自动创建 alias remote `db-local-<space>`（目录是零复制视图）。不写远端即可与挂载、同步组合。

## OneDrive

不走 `storage add`：`drivebridge auth --auto` → `drivebridge setup`。唯一具备直链与 Scaena 参考交接的后端。
