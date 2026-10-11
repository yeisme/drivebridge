## ADDED Requirements

### Requirement: Bounded Key row actions
Key 目录 SHALL 保持内容在各自单元格内，提供单一搜索入口，名称允许多行，次要动作收纳进菜单。菜单 SHALL 保留原权限判定、确认和密钥展示边界。

#### Scenario: Operate a key in a narrow table
- **WHEN** 用户在窄视窗打开 Key 行的更多菜单
- **THEN** 仍可访问被授权的轮转、禁用、启用、吊销、删除和重命名，动作不覆盖身份或状态列
