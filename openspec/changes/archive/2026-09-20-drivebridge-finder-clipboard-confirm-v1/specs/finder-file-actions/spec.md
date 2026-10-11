## ADDED Requirements

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
