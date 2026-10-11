## Why

受管网盘的文件身份在 S3。OneDrive 不是空间存储，而是回存目标。Finder 里复制只发生在 S3 空间内，回存仍藏在文件检查器。generic rclone 继续只在本机面。

## What Changes

- 保持 `POST /files/{id}/copy` 目标为受管 S3。不在服务端引入 rclone generic 连接。
- 新增 `POST /api/v1/directories/export`：把文件夹前缀下的文件回存到已连接的 OneDrive。
- Finder 增加「复制到 OneDrive」，文件走既有 export，文件夹走新前缀作业。

## Capabilities

### New Capabilities
- `directory-export`：目录前缀回存 OneDrive。

### Modified Capabilities
- `finder-file-actions`：复制到 OneDrive。
