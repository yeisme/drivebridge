# directory-prefix-ops Specification

## Purpose
TBD - created by archiving change drivebridge-directory-prefix-ops-v1. Update Purpose after archive.
## Requirements
### Requirement: 目录前缀复制
系统 SHALL 提供 `POST /api/v1/directories/copy`，将源文件夹下未回收文件与空目录记录复制到目标路径。`Idempotency-Key` 必填。目标须为受管 S3。SHALL NOT 覆盖已有路径。超过 1000 个文件 SHALL 返回 `directory_too_large`。根路径 SHALL 返回 `invalid_directory`。

#### Scenario: 复制含文件的文件夹
- **WHEN** 源目录含文件和空子目录，目标路径空闲
- **THEN** 目标出现相同相对结构的新文件身份，源文件保持不变

#### Scenario: 目标文件夹重名
- **WHEN** `conflict` 为 rename 或省略，且目标文件夹路径已被占用
- **THEN** 使用 `name (2)` 作为新文件夹名并复制到其下

### Requirement: 目录前缀移动
系统 SHALL 提供 `POST /api/v1/directories/move`，在同一空间内重写前缀下文件路径与 Directory 行，保留文件身份。SHALL NOT 将目录移动到自身或子路径。

#### Scenario: 移入自身
- **WHEN** 目标路径等于源路径或其子路径
- **THEN** 返回 `invalid_directory` 且不改写任何路径

### Requirement: 目录前缀回收站
系统 SHALL 提供 `POST /api/v1/directories/trash`，将前缀下文件移入回收站，并移除该前缀的 Directory 记录。SHALL NOT 永久删除对象。

#### Scenario: 回收文件夹
- **WHEN** 用户回收一个含文件的目录
- **THEN** 活动浏览不再列出该目录，其中文件 `trashed=true` 且可按文件恢复

