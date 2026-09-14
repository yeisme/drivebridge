## ADDED Requirements

### Requirement: 本机只读存储浏览

本机只读视图 SHALL 支持按空间浏览文件，实时列表由持有 state 锁的 owner 经 insights socket 委派执行，owner 缺席或拒绝时 SHALL 降级为已观察索引并如实说明原因。

#### Scenario: owner 运行时的实时目录
- **WHEN** owner 以 `--enabled --insights` 运行且 view 请求某空间目录
- **THEN** 返回实时列表（`coverage=live_listing`），观察结果进入索引；超过 500 条时截断并标记 `partial`

#### Scenario: owner 缺席的诚实降级
- **WHEN** insights socket 不存在、owner 未启用文件操作或列表失败
- **THEN** 视图不伪造实时结果，按已观察索引呈现并显示对应原因；不撞 state 锁、不新增写入口

#### Scenario: 路径与越界
- **WHEN** 请求携带 `..`、绝对路径、未 Clean 路径或未知空间
- **THEN** 返回 `path_not_allowed`/`space_not_allowed`，不触碰远端

#### Scenario: 目录发现不越出索引
- **WHEN** 远端存在从未被观察过的子目录
- **THEN** 该目录不出现在聚合导航中；目录聚合只来自已观察索引的 Dir 字段
