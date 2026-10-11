## ADDED Requirements

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
