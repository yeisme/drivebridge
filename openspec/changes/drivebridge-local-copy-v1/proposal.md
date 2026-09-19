## Why

`drivebridge view` 的 Finder 已能选中文件夹并调用 copy，但本机 adapter 没有 copy，菜单只能禁用。

## What Changes

- `/files/action` 增加 `copy`、`copy_directory`、`move_directory`、`trash_directory`。
- 文件复制优先走 Provider 服务端 Copy，否则 Open+Upload。永不覆盖，重名 `name (2).ext`。
- 目录复制/移动按前缀快照，最多 1000 个文件；不能移入自身。
- 网页 LocalWorkspace adapter 接上这些动作。

## Capabilities

### Modified Capabilities
- `local-file-experience`：本机复制与目录前缀操作。
