## ADDED Requirements

### Requirement: Two-level drive navigation

客户端 SHALL 先展示可访问的个人与团队网盘，再提供文件工作区操作。

#### Scenario: User enters a drive

- **WHEN** 用户选择一个可访问的网盘卡片
- **THEN** 以对应存储及授权上下文打开文件工作区

### Requirement: Independent sidebar collapse

客户端 SHALL 支持主导航和目录树独立折叠，并保留可访问的恢复控件。

#### Scenario: User collapses the directory tree

- **WHEN** 用户操作目录树折叠按钮
- **THEN** 文件工作区扩展，并保留展开目录树按钮

### Requirement: Key identity visibility

客户端 SHALL 独立展示 Key 的稳定标识、所属用户、状态和可见团队关系，不与凭据明文混淆。

#### Scenario: User edits a key label

- **WHEN** 有权限的用户修改 Key 名称
- **THEN** 展示新名称，所属用户不变且不显示凭据明文

### Requirement: Honest dashboard data

Dashboard SHALL 使用真实可见的资源、操作及文件数据，明确展示空白或不可用状态。

#### Scenario: Team service is disabled

- **WHEN** 服务报告团队治理未启用
- **THEN** 个人工作区仍可使用，且不展示虚构的团队成员关系
