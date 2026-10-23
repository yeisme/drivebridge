# DriveBridge 可选存储后端（rclone × OpenList）

根级设计计划。本轮只冻结方案与规格，不改代码。

**2026-09-19 产品排序收口**：用户决定一等网盘只做 OneDrive / S3 / Google Drive。OpenList、NAS、Dropbox 等不再作为当前产品承诺。能力矩阵诚实规则仍以本包为准；消费者绑定与 Pinax 远程笔记见 [`drivebridge-consumer-binding-v1`](../../../../../openspec/changes/drivebridge-consumer-binding-v1)。本包任务 2.1/2.3（OpenList runbook 与 REST 直链）改为 not-now；gdrive 晋升改由 consumer-binding Wave 1 handoff。

## 结论速览

| 方案 | 后端 | 交付 | 新代码 |
|---|---|---|---|
| A | rclone 直连多后端（gdrive/dropbox/box/pcloud/s3/smb/sftp/webdav/local） | 已可用 | 零 |
| B | OpenList WebDAV 聚合（百度/阿里/115/夸克等国内网盘） | deliver-now（runbook） | 零 |
| C | Google Drive 一等后端晋升（copy + MD5 钉版） | retain-next | 小 |
| D | 自托管 S3/MinIO 作内网 CAS 卷 | 已可用（用法） | 零 |
| E | OpenList REST 直链 provider（v2 直链） | retain-next，默认关闭 | 中 |
| F | NAS 原生 local/SMB/SFTP/alias | 已可用 | 零 |

## 关键文档

- 设计与能力矩阵：[design.md](design.md)
- 规格：[specs/drivebridge-storage-backends/spec.md](specs/drivebridge-storage-backends/spec.md)
- 与内网 CAS/挂载的关系：`openspec/changes/drivebridge-intranet-cas-mount-v1/`
