## Why

文件夹单击仍会进入，剪贴板只认文件。目录前缀作业落地后，Finder 必须能选中文件夹并提交 copy/move/trash。

## What Changes

- 单击选中文件夹，双击 / Enter / → / 打开按钮进入。
- 剪贴板同时持有文件和目录；粘贴调用 `directories/copy|move`。
- 无目录 adapter 的本机连接禁用目录复制。
- 更新 `finder-actions` e2e：进入文件夹改为双击。

## Capabilities

### Modified Capabilities
- `finder-file-actions`：文件夹选择与目录剪贴板。
