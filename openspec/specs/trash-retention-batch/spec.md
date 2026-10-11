# trash-retention-batch Specification

## Purpose
TBD - created by archiving change drivebridge-key-delete-trash-batch-v1. Update Purpose after archive.
## Requirements
### Requirement: 回收站保留期与定时清理

移入回收站 SHALL 记录 `trashed_at`。空间 SHALL 有 `trash_retention_days`，缺省 30，可设 1–365。服务 SHALL 在启动时及周期性扫描中，对超过保留期的回收站文件执行与手动永久删除相同的清理。Agent SHALL NOT 触发永久删除。

#### Scenario: 到期自动清理
- **WHEN** 文件已在回收站超过该空间保留期
- **THEN** 扫描后该文件不再出现在回收站，对象引用被释放

### Requirement: 回收站批量恢复与永久删除

服务 SHALL 接受最多 100 个文件 ID 的批量恢复和批量永久删除。永久删除仍需确认头。部分失败 SHALL 按文件返回错误码，已成功项保留。

#### Scenario: 批量恢复
- **WHEN** 用户对回收站中多个本人文件提交 batch/restore
- **THEN** 成功项 `trashed=false`，失败项带稳定错误码

