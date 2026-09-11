## Why

个人文件不只住在 OneDrive：NAS（SMB/WebDAV/SFTP）、对象存储、Dropbox/Google Drive 都是真实目标。rclone 已内嵌（v1.70.3 全后端），Provider 接口本就基于 rclone fs；只需把"OneDrive 特化"降级为"后端之一"，即可把传输面扩展到任意 rclone 后端。

## What Changes（设计，实现归后续波）

Space 增 `Backend` kind：`onedrive`（默认，保持全部现有语义）与 `generic`（任意 rclone 后端，仅传输面）。generic 空间支持 list/stat/upload/download/operation 恢复；`Access`（临时直链）与 `prepare_reference`（Scaena 参考交接）明确拒绝——参考交接保持 OneDrive 专属。网络存储定位为消费面（作为空间目标），serve/mount（对外暴露 WebDAV/FUSE）延期。软链接默认拒绝，空间级显式 opt-in 才跟随。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-local-files`：新增 Backend kind capability matrix 与 Symbolic link policy 需求。

## Impact

space-add 增 `--backend` 与后端参数透传（凭据走 rclone 配置，不进 state 明文）；capabilities 资源按 kind 报能力矩阵；引用/交接合同零变化（generic 引用仍为稳定 `drivebridge://file/<id>`，但 handoff 不可用）。实现波不动 OneDrive 现有路径与测试。
