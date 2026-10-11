## ADDED Requirements

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
