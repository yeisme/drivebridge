## ADDED Requirements

### Requirement: Soft delete and restore
软删除 SHALL 限制在空间内且可恢复：trash 移入空间内隔离目录并保留引用可查；restore 移回原位且同名冲突加时间戳后缀不覆盖；永久删除不进入 Agent 面。

#### Scenario: Trash keeps the reference queryable
- **WHEN** agent 软删除一个引用
- **THEN** stat 返回 trashed 状态；download 与参考交接以固定码拒绝；配额仍如实占用

#### Scenario: Restore never overwrites
- **WHEN** 恢复时原位已存在同名文件
- **THEN** 恢复为带时间戳后缀的名称，双方文件都保留

#### Scenario: Permanent deletion stays with the owner
- **WHEN** 通过 MCP/Agent 请求永久删除
- **THEN** 拒绝并指向用户专属入口（网页回收站或带显式 owner 确认旗标的 CLI）
