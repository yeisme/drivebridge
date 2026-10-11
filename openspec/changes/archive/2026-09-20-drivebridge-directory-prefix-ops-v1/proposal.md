## Why

Finder 已能复制单个文件，但文件夹仍是“点一下就进入”的导航，没有前缀 copy/move/trash。网页不能自己翻页拼递归复制。

## What Changes

- `POST /api/v1/directories/copy|move|trash`：按源路径前缀操作。copy 为可 resume 的 `kind=directory_copy` 作业；move/trash 为同空间元数据操作。
- copy 复用文件 copy（CAS、不覆盖）；目标文件夹名冲突默认 `资料 (2)`。
- 禁止把目录移动到自身或子路径。超过 1000 个文件返回 `directory_too_large`。
- 不改 file copy/move/trash 路径。MCP 本切片不加目录工具。

## Capabilities

### New Capabilities
- `directory-prefix-ops`：目录前缀复制、移动、回收站。

## Impact

split-owner。网页只提交前缀作业。本机 view 无对应 adapter 时禁用目录复制。不提供递归永久删除。
