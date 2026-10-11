# finder-file-actions Specification

## Purpose
定义 Finder 的文件、目录、剪贴板、新建、移动和分享操作入口，以及键盘、右键和窄屏菜单的等价行为。所有动作遵守当前权限、后端能力和可恢复操作合同。

## Requirements

### Requirement: 文件上下文菜单
客户端 SHALL 在空白区、文件、文件夹及目录树提供符合权限和后端能力的上下文菜单，并支持键盘和更多按钮入口。

#### Scenario: 移动端菜单
- **WHEN** 点击文件更多按钮或使用 Shift+F10
- **THEN** 菜单保持在视口内，可通过方向键选择及 Escape 关闭

### Requirement: 可恢复的新建流程
客户端 SHALL 展示新建目标路径，支持显式多级目录及文本文件，不在重名或失败时覆盖旧内容或清空输入。

#### Scenario: 重名后继续
- **WHEN** 创建或重命名遇到已有路径
- **THEN** 保留输入和内容，用户修改名称后可重新提交

### Requirement: 真实元数据
客户端 SHALL 展示实际文件和版本信息，目录统计明确限定当前页，不冒充递归总量。

#### Scenario: 空文件
- **WHEN** 在受管网页创建空文本文件
- **THEN** 文件大小为零，上传完成状态可见，并可从列表查看元数据

### Requirement: 文件剪贴板
客户端 SHALL 提供复制、剪切、粘贴和创建副本。复制与创建副本调用服务端 copy；同网盘剪切粘贴调用现有 move。SHALL NOT 覆盖已有路径。无 copy 能力时 SHALL 禁用并说明原因。跨网盘剪切 SHALL 禁用。

#### Scenario: 复制后粘贴到另一目录
- **WHEN** 用户复制文件并在另一目录粘贴
- **THEN** 目标出现新文件，源文件仍在，内容与固定源版本一致

#### Scenario: 同目录创建副本
- **WHEN** 用户对当前文件选择创建副本
- **THEN** 出现自动改名的新文件，原文件不变

### Requirement: Finder 快捷键
客户端 SHALL 在文件区提供复制/剪切/粘贴/创建副本、当前页全选、Delete 打开回收站确认、F2 重命名、方向键焦点与 Shift 范围选择。焦点在输入框、对话框、菜单或媒体控件时 SHALL NOT 拦截这些快捷键。

#### Scenario: 搜索框中的复制
- **WHEN** 焦点在搜索框并按下 Ctrl/Cmd+C
- **THEN** 不把当前选中文件写入内部剪贴板

### Requirement: 危险操作二次确认
移入回收站、永久删除和撤销本机分享 SHALL 使用同一确认对话框，快捷键不得跳过。永久删除 SHALL 先展示 usage 影响，用户确认后仍发送服务端确认头。

#### Scenario: Delete 键打开回收站确认
- **WHEN** 已选择文件并按下 Delete
- **THEN** 出现回收站确认；取消后文件仍在当前目录

### Requirement: 文件夹选择
客户端 SHALL 单击选中文件夹，双击、Enter、方向右或打开控件进入该目录。SHALL NOT 用单击进入替代选中。

#### Scenario: 单击不进入
- **WHEN** 用户单击一个文件夹名称
- **THEN** 该文件夹被选中，当前目录路径不变

#### Scenario: 双击进入
- **WHEN** 用户双击已显示的文件夹
- **THEN** 浏览该文件夹内容

### Requirement: 目录剪贴板
客户端 SHALL 把选中的文件夹放入内部剪贴板，粘贴时调用服务端目录 copy 或同网盘 move。本机无目录 copy 时 SHALL 禁用并说明原因。

#### Scenario: 复制文件夹后粘贴
- **WHEN** 用户复制一个含文件的文件夹并在另一目录粘贴
- **THEN** 目标出现新文件夹和其中文件，源文件夹仍在

### Requirement: 复制作业进入传输中心
客户端 SHALL 把文件 copy 与目录 copy 的 Operation 写入传输中心，包括 CAS 秒传完成的记录。目录复制 SHALL 展示已完成/总文件数。

#### Scenario: 秒传复制可见
- **WHEN** 用户复制文件且服务端 `blob_reused`
- **THEN** 传输中心出现已完成的复制记录

### Requirement: 目录树选择粘贴或移动目标
客户端 SHALL 为“移动”和“粘贴到…”提供当前网盘目录树。Ctrl/Cmd+V SHALL 仍粘贴到当前目录。

#### Scenario: 移动到所选文件夹
- **WHEN** 用户打开移动对话框并选择一个文件夹
- **THEN** 选中项移动到该文件夹，而不是要求手填路径

### Requirement: 撤销上一次移动
客户端 SHALL 记住本页会话中上一次移动或剪切粘贴，并可用撤销或 Ctrl/Cmd+Z 移回。SHALL NOT 撤销复制。

#### Scenario: 撤销剪切粘贴
- **WHEN** 用户剪切文件并粘贴到另一目录后选择撤销
- **THEN** 文件回到剪切前的路径

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

### Requirement: 新建表单保持可读的选项布局
客户端 SHALL 将新建表单限制在桌面 480px 与移动端可用宽度内，保存位置支持换行，勾选项与标签横向对齐。

#### Scenario: 创建多级目录
- **WHEN** 用户输入带斜杠名称并勾选创建多级目录
- **THEN** 校验允许提交，创建后打开选项保持原有行为
