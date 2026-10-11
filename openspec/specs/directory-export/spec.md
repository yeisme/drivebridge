# directory-export Specification

## Purpose
TBD - created by archiving change drivebridge-onedrive-export-finder-v1. Update Purpose after archive.
## Requirements
### Requirement: 目录前缀回存 OneDrive
系统 SHALL 提供 `POST /api/v1/directories/export`，将源文件夹下未回收文件导出到指定 OneDrive 连接。`Idempotency-Key` 必填。SHALL NOT 把 OneDrive 当作空间对象存储。generic rclone SHALL NOT 作为此接口目标。

#### Scenario: 导出含文件的文件夹
- **WHEN** 用户将含一个文件的目录导出到已验证的 OneDrive 连接
- **THEN** 作业 completed，该文件出现在 OneDrive，源空间文件不变

#### Scenario: 幂等重放
- **WHEN** 使用同一 Idempotency-Key 重放相同导出
- **THEN** 返回原作业，不重复上传

