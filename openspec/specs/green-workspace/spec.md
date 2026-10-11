# green-workspace Specification

## Purpose
定义墨绿色文件工作区的网盘目录、列表详情、侧栏折叠、身份展示和资源概览。摘要只展示当前可访问且真实可解释的数据，并保留进入完整文件工作区的路径。

## Requirements

### Requirement: Two-level drive navigation

客户端 SHALL 先展示可访问个人与团队网盘，再进入原文件工作区；目录 SHALL 提供列表详情和保留的卡片模式。未指定布局时默认列表详情，单击选择、双击或 Enter 打开，每项有明确打开动作；卡片模式保留单击进入。

#### Scenario: User enters a drive

- **WHEN** 用户选择一个可访问的网盘卡片
- **THEN** 以对应存储及授权上下文打开文件工作区

#### Scenario: User inspects a list item

- **WHEN** 用户在列表详情模式选择网盘
- **THEN** 展示该网盘可见的归属、存储、连接、用量、配额与权限摘要，不立即进入文件区或修改资源

#### Scenario: Directory adapts to content width

- **WHEN** 目录内容宽度至少 880px，或缩窄到不足 880px
- **THEN** 宽布局使用列表加 320px 详情，窄布局用全宽列表及按需详情抽屉，不压缩成不可读双栏

#### Scenario: Filter or permission change invalidates selection

- **WHEN** selected 参数对应项目不在当前可见且匹配筛选的结果中
- **THEN** 清除失效选择，不按参数读取隐藏资源；进入文件区再返回保留仍有效的查询和选择

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

### Requirement: Key identity visibility

客户端 SHALL 独立展示 Key 的稳定标识、所属用户、状态和可见团队关系，不与凭据明文混淆。

#### Scenario: User edits a key label

- **WHEN** 有权限的用户修改 Key 名称
- **THEN** 展示新名称，所属用户不变且不显示凭据明文

### Requirement: Honest dashboard data

Dashboard SHALL 使用真实可见的资源、操作及文件数据，明确展示空白或不可用状态。概览 SHALL 使用紧凑指标区与可读列表，未知值不得补零，空状态不得为保持卡片高度制造大块留白。

#### Scenario: Team service is disabled

- **WHEN** 服务报告团队治理未启用
- **THEN** 个人工作区仍可使用，且不展示虚构的团队成员关系

#### Scenario: Compact overview keeps actions

- **WHEN** 用户在窄内容区查看概览
- **THEN** 指标按内容宽度变为两列，存储容量右对齐，传输和最近文件保留原打开动作与简洁空状态
