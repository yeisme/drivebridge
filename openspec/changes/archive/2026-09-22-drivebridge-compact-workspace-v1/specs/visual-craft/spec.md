## MODIFIED Requirements

### Requirement: Shared surface tokens

客户端 SHALL 在共享主题入口提供表面、文字、边框、投影与半径语义令牌，现有布局别名 SHALL 指向同一来源。Insights、Finder、表格与表单 SHALL 引用这些令牌，不得新增独立主题或让分隔线回退到文字 currentColor。

#### Scenario: Insight card uses shell tokens

- **WHEN** 用户打开资源态势
- **THEN** 指标与图表区域的背景、边框和投影来自共享语义令牌，深浅主题同步切换

#### Scenario: Dark table separators do not become bright lines

- **WHEN** 用户在深色主题打开用户目录或团队权限表
- **THEN** 普通行分隔线使用共享边框色，不呈现白色或正文色亮线；键盘焦点仍独立可见

## ADDED Requirements

### Requirement: Control size and content spacing are independent

客户端 SHALL 将桌面单行控件统一为 32px，并给业务表格至少 48px 行高、单元格上下各至少 8px 与左右各至少 12px 留白。双行记录 SHALL 至少 64px 且随内容增高。复合文件项、预览和多行内容 SHALL NOT 继承单行控件固定高度。

#### Scenario: Inline permission controls keep breathing room

- **WHEN** 权限表行包含 32px 下拉控件
- **THEN** 控件上下各有至少 8px 内容留白，文字和控件均不触及行边界

#### Scenario: Narrow table remains readable

- **WHEN** 在 872px 或 390px 窗口查看含长姓名、状态与操作的表格
- **THEN** 不压缩正文到 14px 以下或裁切多行内容；仅表格自身可以水平滚动，工具栏距外边框至少 16px

### Requirement: Form spacing and explicit actions

客户端 SHALL 保留可见字段标签，标签与控件间至少 8px、字段组间至少 16px、字段区到操作区至少 24px、取消与提交间至少 8px。字段错误 SHALL 就近展示，失败保留输入；触屏交互目标 SHALL 至少 44×44px。

#### Scenario: Storage assignment form actions do not touch inputs

- **WHEN** 用户打开团队“分配已有存储”表单
- **THEN** 下拉、取消与保存处于独立区域，有规定间距且全部可达，提交不会因误触相邻按钮重复执行

#### Scenario: Failed submission preserves user input

- **WHEN** 提交返回校验或权限错误
- **THEN** 表单保留已填内容，在当前弹层显示错误，用户可修改重试或取消
