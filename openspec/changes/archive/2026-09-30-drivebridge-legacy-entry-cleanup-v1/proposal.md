## Why

用户在完成单仓合并后明确要求“清理原来的项目”。三个旧目录仍以软链接出现在工作区，部分脚本、文档和本地 Git 配置继续使用旧 owner，容易误入旧路径开展工作。

## What Changes

- 撤销此前保留一个发布周期的目录兼容策略，移除三个工作区旧软链接及本地子模块注册残留。
- 更新当前文档、active OpenSpec 与 MCP 校验路径。任务编号、状态及历史证据不变。
- 保留合并后的 CLI/MCP、Server、Web 源码、原 Git 历史备份和归档记录。

## Capabilities

### New Capabilities
- `drivebridge-legacy-entry-retirement`: 旧项目入口退役及保全验收。

## Impact

唯一开发目录为 `cli/drivebridge`。旧的绝对路径和 cd 命令不再可用，需切换到新目录。仅清理本地工作区，不变更已部署服务、外部配置、协议、存储状态或前端实现。
