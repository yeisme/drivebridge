## Why

Finder 已有右键菜单、拖拽移动和回收站，但“复制”只写引用，没有文件剪贴板；快捷键几乎只覆盖预览；删除确认强弱不一。用户需要可键盘操作的文件管理器。

## What Changes

- 内部剪贴板：复制/剪切/粘贴/创建副本；粘贴调用服务端 copy 或现有 move。
- Finder 快捷键表、焦点环、当前页全选与范围选择；输入框不拦截。
- 统一二次确认：回收站、永久删除（usage + 输入校对）、本机分享撤销不再使用 `window.confirm`。
- 无 copy 能力的本机连接诚实禁用复制/粘贴；跨网盘剪切禁用。

## Capabilities

### Modified Capabilities
- `finder-file-actions`：剪贴板、快捷键、确认。

## Impact

split-owner：复制规则在 DriveBridge Server。网页不覆盖路径、不把确认当成服务端门。文件夹递归复制 retain-next。
