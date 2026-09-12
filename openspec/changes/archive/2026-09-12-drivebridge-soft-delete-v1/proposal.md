## Why

目录组织落地后，agent 迟早要整理文件；现行合同"禁止删除"把整理全部推给用户。用户已选定**软删除+恢复**档：agent 可 trash/restore（空间内），永久删除仍只能用户本人。

## What Changes（设计，实现归后续波）

- MCP 新工具 `trash(file_ref)` / `restore(ref)`：软删除=远端移入空间内 `.drivebridge-trash/<date>/`（OneDrive=Graph move，generic=rclone server-side move，不可用则受控下载+上传后删源并如实标注）；File 行标 `trashed_at`，stat 返回 trashed 状态；restore=移回原目录（同名冲突→带时间戳后缀，不覆盖）。
- 引用语义：trashed 文件的稳定引用仍可 stat（状态=trashed）；download/prepare_reference 拒绝（`file_trashed`）。
- 永久删除：MCP/Agent 无此工具；用户门=网页回收站（backend 已有）与 CLI `purge --ref`（要求 `--i-am-the-owner` 显式确认旗标）。
- AGENTS.md 边界改写："禁止删除"→"禁止永久删除；软删除仅限空间内且可恢复"。

## Capabilities

### New Capabilities
无（drivebridge-local-files 增量需求，实现波落 delta）。

### Modified Capabilities
- `drivebridge-local-files`：Soft delete and restore 需求——本波仅设计。

## Impact

trash 后空间配额仍占用（如实报告）；`.drivebridge-trash` 不参与 list/search 结果但计入配额；无自动过期清理（与 discovery-v1 的手动 gc 对齐）。
