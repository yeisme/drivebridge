## MODIFIED Requirements

### Requirement: Independent sidebar collapse

客户端 SHALL 支持主导航和目录树独立折叠，并保留可访问的恢复控件。主导航展开时 SHALL 按工作台、协作、系统分组；折叠后的主导航 SHALL 显示产品标与互不相同的入口图标，不得用同一 lucide 图形表示不同页面。

#### Scenario: User collapses the directory tree

- **WHEN** 用户操作目录树折叠按钮
- **THEN** 文件工作区扩展，并保留展开目录树按钮

#### Scenario: User collapses the main sidebar

- **WHEN** 用户折叠主导航或使用 Ctrl/Cmd+B
- **THEN** 每个入口仍可通过图标与 `aria-label` 辨认，品牌位仍为产品标

#### Scenario: Navigation groups remain after collapse restore

- **WHEN** 用户展开主导航
- **THEN** 入口仍位于工作台、协作或系统组内，而不是单列平铺
