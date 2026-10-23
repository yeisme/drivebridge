## Why

DriveBridge 的稳定引用、传输恢复与能力面目前只对 OneDrive 提供"完整能力"，其余后端共享一个 transfer-only 的 `generic` 桶。用户实际存储分布在：Google Drive、Dropbox、S3 兼容族（MinIO/COS/OSS/R2/B2）、NAS（SMB/SFTP/local），以及 rclone 不支持的国内网盘（百度/阿里/115/夸克，经由自托管 OpenList 聚合暴露 WebDAV）。

**范围变更（2026-09-19）**：一等产品面收缩为 OneDrive、S3、Google Drive。见根包 `drivebridge-consumer-binding-v1`。本包仍冻结能力矩阵与身份纪律；OpenList/NAS 一等化不再是当前交付。

需要一份诚实的能力矩阵与接入方案：哪些后端今天可用、哪些值得能力晋升、直链/交接永远不被伪造。

## What Changes

- 冻结按后端的能力矩阵：transfer / server_side_copy / native_digest / version_pin / direct_url / reference_handoff。
- 冻结身份纪律：blob 身份恒为 DriveBridge 计算的明文 SHA-256；provider 原生哈希（如 Google Drive MD5）只做版本变更检测，不授权、不当 blob 身份。
- 给出六个可选存储方案（A–F）与各自命令路径；B/E 以自托管 OpenList 为聚合器。
- 明确第三方聚合器信任边界：自托管、钉版本、专用受限账号、凭据 0600、直链默认关闭。
- 区分两个角色：实例拥有的**平台 blob 卷**（CAS）与用户拥有的**个人空间后端**；不混用。

本轮不改代码。实现归后续 owner change。

## Capabilities

### New Capabilities

- `drivebridge-storage-backends`：多后端能力矩阵、版本钉住与身份纪律。

### Modified Capabilities

无删除。现有 `drivebridge-local-files`（generic 行为）与 OneDrive 交接语义保持；方案 C/E 是加法晋升。

## Impact

设计与验收合同归根 OpenSpec。实现波预计落 `cli/drivebridge`（本机桥后端能力标注、gdrive 钉版、OpenList provider）与 `cli/drivebridge/server`（平台卷文档）；各自建 owner change 后再动代码。
