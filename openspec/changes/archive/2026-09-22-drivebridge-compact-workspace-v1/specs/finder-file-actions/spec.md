## ADDED Requirements

### Requirement: Unified file command strip

Finder SHALL 把目录导航、文件视图与专注放在首行，搜索独立一行，文件操作另成一行。新建文件和文件夹 SHALL 合入新建菜单；桌面同排控件为 32px且高度差不超过 1px。内容宽度不足 640px 时 SHALL 合并筛选与排序并把分享收进更多，所有原动作仍可达。

#### Scenario: Narrow command strip

- **WHEN** 957px 窗口中展开侧栏和目录树导致 Finder 内容宽度不足 640px
- **THEN** 显示上传、新建、筛选与排序和更多，不出现零散换行；新建、分享和原快捷键保留权限判定

#### Scenario: Compound surfaces keep natural height

- **WHEN** 用户切换分栏或画廊
- **THEN** 文件行与缩略项不受工具栏 32px 高度规则限制，仍可阅读名称并打开文件

### Requirement: Focused drive command preserves context

Finder SHALL 移除顶部重复位置描述行，将专注放在视图切换旁并作视觉分隔。专注 SHALL 在新标签页打开，保留当前网盘、租户、团队和管理访问范围；返回完整工作区只移除专注参数。

#### Scenario: Open and return from focused drive

- **WHEN** 用户从团队或管理网盘选择专注，再返回完整工作区
- **THEN** 网盘与授权上下文保持原值，返回动作在专注窗口中可达，其他四种视图不被移除
