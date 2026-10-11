## Why

复制已能完成，但 CAS 秒传不进传输中心，移动仍靠手填路径，剪切后无法撤销。

## What Changes

- 文件 copy 与 `directory_copy` 无论是否秒传都进入传输中心；文件夹复制显示文件进度。
- 移动和“粘贴到…”用目录树选择目标，Ctrl+V 仍粘贴到当前目录。
- 会话内可撤销上一次移动或剪切粘贴（Ctrl+Z），不撤销复制。

## Capabilities

### Modified Capabilities
- `finder-file-actions`：粘贴作业可见、目标选择、撤销移动。
