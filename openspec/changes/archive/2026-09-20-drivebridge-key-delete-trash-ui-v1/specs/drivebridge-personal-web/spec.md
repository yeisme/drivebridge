## ADDED Requirements

### Requirement: Key 管理支持删除
网页 SHALL 在 Key 行展示吊销与删除（当服务端返回 can_revoke/can_delete）。删除须确认。最后一把有效 Key SHALL NOT 出现删除按钮。

#### Scenario: 删除备用 Key
- **WHEN** 用户新建第二把 Key 并确认删除
- **THEN** 该行变为已归档/已吊销，不能再复制明文

### Requirement: Key 管理支持禁用和启用
网页 SHALL 在 can_disable 时提供禁用确认，在 can_enable 时提供启用。禁用中的 Key 显示「已禁用」，且不能复制明文。

#### Scenario: 禁用后再启用
- **WHEN** 用户禁用一把备用 Key 再启用
- **THEN** 状态从已禁用回到有效

### Requirement: 回收站批量与定时清理
回收站 SHALL 支持全选、恢复所选、永久删除所选，并展示空间自动清理天数。

#### Scenario: 批量恢复
- **WHEN** 用户全选回收站文件并恢复所选
- **THEN** 这些文件回到网盘列表
