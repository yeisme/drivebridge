# 可选存储后端：rclone × OpenList

**范围变更（2026-09-19）**：产品排序不再按 A–F 全量推进。一等个人空间后端以 [`drivebridge-consumer-binding-v1`](../../../../../openspec/changes/drivebridge-consumer-binding-v1/design.md) 为准（OneDrive / S3 / Google Drive）。本文件的能力矩阵、身份纪律、平台卷与个人空间分家仍然有效；方案 B/E/F 降为 generic 逃生舱与 not-now。

## 基线

- 本机桥内嵌 rclone，后端清单 `internal/bridge/backends.go`：alias/local/box/dropbox/**drive**(Google)/pcloud/s3/smb/sftp/webdav + OneDrive。新增 rclone 后端=补一行 import。
- 能力现状：OneDrive = 传输 + 临时直链 + Scaena 参考交接；其余全为 `generic` = 仅传输（`access_unsupported` / `handoff_not_available`）。
- 版本钉住：OneDrive = item+ETag；generic = 大小+mtime，漂移报 `file_version_changed`。
- `ReuseUpload`：同空间同 SHA-256 且 provider 能 server-side copy 时零字节；否则整传。
- 身份纪律：blob = DriveBridge 计算的明文 SHA-256。

## 能力矩阵（目标）

| 后端 | 接入 | transfer | server_side_copy | native_digest | version_pin | direct_url | reference_handoff |
|---|---|---|---|---|---|---|---|
| OneDrive（现状） | 内置 | ✅ | ✅ Graph copy | ❌ | item+ETag | ✅ | ✅ |
| A. rclone generic | `--backend generic` | ✅ | 按 provider | provider 自报 | 大小+mtime | ❌ | ❌ |
| B. OpenList（WebDAV） | generic + rclone webdav | ✅ | ❌ | ETag 可选 | ETag 否则大小+mtime | ❌ v1 | ❌ |
| C. Google Drive 晋升 | named `gdrive` | ✅ | ✅ 同账号 | **MD5** | file_id+md5+mtime | ❌ v1 | ❌ |
| D. 自托管 S3 | generic 或平台卷 | ✅ | ❌ | ETag 不可靠→不用 | 大小+mtime | ❌ | ❌ |
| E. OpenList 直链（v2） | named `openlist`（REST） | ✅ | ❌ | ❌ | API ETag | ⭕ 一次性 | ❌ |
| F. NAS local/SMB/SFTP/alias | generic | ✅ | alias ❌ | ❌ | 大小+mtime | ❌ | ❌ |

`❌` 必须照实返回 `access_unsupported` / `handoff_not_available`，不降级伪装。能力经 `capabilities` 的 `backend_capabilities` 如实上报；未实现的键视为 false。

## 方案

### A. rclone 直连多后端（已可用，零代码）

```bash
drivebridge space add --space gdrive --backend generic \
  --remote '<rclone-remote>' --remote-path '<子目录>' \
  --local-root /absolute/path --max-bytes 1073741824
```

覆盖 Google Drive、Dropbox、Box、pCloud、任意 S3、SMB、SFTP、WebDAV、本地目录。纪律不变：先 `list` 再 act；symlink 默认拒；永不覆盖；mtime 漂移诚实报 `file_version_changed`。大文件参考交接不可用——需要 Provider 直链的场景仍走 OneDrive 或 Server 解密流。

### B. OpenList WebDAV 聚合（deliver-now，零代码）

动机：rclone 不支持百度/阿里/115/夸克等国内网盘；[OpenList](https://github.com/OpenListTeam/OpenList)（AList 社区分支）把数十种驱动统一成 WebDAV/REST。

接入（自托管 OpenList 为前置）：

1. 部署 OpenList 于内网/Tailscale 后，建**专用受限账号**（只授需要目录的可见/可写权限），钉住已验收版本。
2. 在自有 rclone 配置加 webdav 节（`vendor = other`，Basic 用户名/密码；不经 OpenList token——token 属 REST API）：
   ```bash
   rclone config create openlist webdav url 'http://<内网地址>:5244/dav' vendor other user '<专用账号>' pass '< obscured>'
   ```
3. 建空间：`drivebridge space add --space cloud --backend generic --remote openlist --remote-path '<OpenList内路径>' --local-root <本机目录> --max-bytes <n>`

边界：OpenList 是第三方聚合器——供应链自担：自托管、钉版本、升级单独验收。国内网盘 mtime/ETag 抖动常见，`file_version_changed` 是诚实行为不是缺陷。大文件走 OpenList 分片，DriveBridge 不越俎代庖。本机 generic 空间无 Server 信封加密，落盘为远端明文——如实告知。

### C. Google Drive 一等后端晋升（retain-next，小代码）

`drive` 已 import，今天走 generic。晋升点：

1. named backend `gdrive`，能力上报 `server_side_copy:true`、`native_digest:"md5"`。
2. 版本钉住升级为 `file_id + md5 + modifiedTime`：消除 mtime 假漂移；md5 变化 → `file_version_changed`。
3. `ReuseUpload` 零字节经 rclone 同账号服务端复制（实现波验证 drive 的 copy feature）。
4. 边界：MD5 只做变更检测与展示；blob 身份仍是 SHA-256。Google Docs（在线文档）不是字节文件，v1 拒绝导出（`unsupported`），不做格式转换。

### D. 自托管 S3/MinIO 作内网 CAS 卷（已可用，用法级）

家庭/实验室首选 MinIO。两个角色必须分清：

- **平台 blob 卷**（实例拥有）：`drivebridge-server platform-connect` 指向 MinIO/COS/OSS/R2/B2，承载数据库+去重后的对象（HTTPS 公网或 Tailscale+TLS；`--verify-isolation` 必跑）。
- **用户个人空间**（用户拥有）：同一 MinIO 也可经 generic 建空间，行为与普通网盘一致。

共享桶隔离沿用既有纪律（前缀作用域子凭据 / 空间信封加密）。

### E. OpenList REST 直链 provider（v2，默认关闭）

OpenList `POST /api/fs/get` 返回 `raw_url`（各网盘签名直链）。若未来要给 OpenList 空间开 `direct_url`，沿用 OneDrive 交接纪律：

- 临时 URL 只在 owner 进程提交时刻获取，用后即弃；不进 MCP 输出/日志/回执/持久化。
- TTL 未知必须如实报（国内网盘直链常常几分钟失效）；域名校验防重定向投毒。
- `prepare_reference` 类交接仍可拒绝：reference_handoff 保持 OneDrive 专属，直到有独立验收。
- 默认关闭，`capabilities` 区分"合同存在/当前可执行"。

### F. NAS 原生（已可用）

local/SMB/SFTP 走 generic；`alias` 后端可为同一物理目录建多个空间视图（零复制）。适合局域网冷层与挂载组合：挂载读 → 本机 CAS → NAS 空间。

## 平台卷 vs 个人空间

| | 平台 blob 卷 | 个人空间后端 |
|---|---|---|
| 拥有者 | Server 实例 | 用户 |
| 配置 | `platform-connect`（一次性，不重绑） | `space add`（可多个） |
| 去重/CAS | ✅ blob volume | ❌（各空间独立对象） |
| 信封加密 | ✅（Server 托管空间） | ❌ 本机 generic 为远端明文 |
| 典型 | MinIO/COS | OneDrive/Google/OpenList/NAS |

## 分期与回滚

1. **deliver-now（本轮规格）**：能力矩阵与身份纪律冻结；A/D 作为 S3/rclone 用法说明保留。
2. **current（改由 consumer-binding Wave 1）**：C gdrive 晋升；S3 个人空间 named 一等。
3. **not-now（2026-09-19 用户收口）**：B OpenList WebDAV 产品化、E OpenList REST 直链、F NAS 一等入口。generic 实现不删除。
4. **not-requested**：公开分享、网盘离线下载、转码、跨实例联邦、把 OpenList/任何后端冒充平台卷。

回滚：B 删空间即可；C 能力键回退即恢复 generic 语义（named backend 是加法，不动存量 generic 空间）；E 关旗标。凭据轮换走各后端自身机制（OpenList 换专用账号密码、S3 换子凭据）。
