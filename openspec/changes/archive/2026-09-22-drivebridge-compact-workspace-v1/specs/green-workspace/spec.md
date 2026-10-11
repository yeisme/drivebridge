## MODIFIED Requirements

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

### Requirement: Honest dashboard data

Dashboard SHALL 使用真实可见的资源、操作及文件数据，明确展示空白或不可用状态。概览 SHALL 使用紧凑指标区与可读列表，未知值不得补零，空状态不得为保持卡片高度制造大块留白。

#### Scenario: Team service is disabled

- **WHEN** 服务报告团队治理未启用
- **THEN** 个人工作区仍可使用，且不展示虚构的团队成员关系

#### Scenario: Compact overview keeps actions

- **WHEN** 用户在窄内容区查看概览
- **THEN** 指标按内容宽度变为两列，存储容量右对齐，传输和最近文件保留原打开动作与简洁空状态
