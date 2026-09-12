## Why

"Agent 云存储工具"定位的第一缺口是工作区组织与直连交互：MCP `list` 只看空间根层（agent 无法组织子目录），CLI 没有本地传输命令（脚本/终端用户必须起 MCP），客户端注册靠手工拼长命令行。

## What Changes

- **MCP 目录工具**：`list` 增可选 `path`（工作区相对目录，逃逸/绝对路径拒绝）；新工具 `mkdir(space_id, path)`（幂等；OneDrive=Graph 逐段建 folder，generic=rclone `Fs.Mkdir`）；`upload` 增可选 `dir`（目标名仍为 `<operation-id>-<name>`，不替换既有文件）。
- **CLI 传输命令**：`upload/download/ls/stat/operation/mkdir` 子命令（cobra，配置面复用）；CLI 是用户本人终端，不设 `--enabled` 门（该门只管 MCP 写入口）；CLI 幂等键缺省确定性派生（space+file+name / ref+out），重试复用原操作。
- **`install --client`**：claude-code（shell 出 `claude mcp add -s user`）/ codex（幂等追加 `~/.codex/config.toml`）/ generic（打印片段）；只写注册项，零 secret。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-local-files`：新增 Workspace directories and transfer CLI 需求。

## Impact

Provider 接口扩展 `List(dir)`/`Mkdir`（OneDrive 走 Graph path 寻址，generic 走 rclone）；操作/恢复/引用合同零变化（目录只是目标命名空间的前缀）；`--enabled` 门语义不变（仅 MCP 面）。
