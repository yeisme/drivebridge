## Why

用户明确要求已登录的最高管理员可以通过输入目标用户名或自己的 Key 确认删除，并能处理最后一位团队管理员或负责人。同时网盘需要完整的新建文本文件路径，包括空文件。

## What Changes

- 新增 confirmation_name 作为已认证管理员的操作确认方式，保留 proof 方式。
- 操作影响返回明确的职责接管清单与摘要；最高管理员显式选择后，在同一事务中接任缺失职责并执行账号操作。
- 现有上传支持零字节，空加密对象仍有最终认证帧。
- 隔离 fixture 支持无进行中任务时复用账号/会话/目录检查点，避免升级预览时重新签发用户 Key；已有文件通过显式保留原存储连接继续访问。

## Capabilities

### Modified Capabilities
- `account-directory`: 用户名确认与显式职责接管。

### New Capabilities
- `empty-file-upload`: 零字节文件沿现有受管上传/存储验证流程完成。

## Impact

split-owner：backend-server/drivebridge 负责鉴权、接管事务、审计及上传；client/drivebridge-client 提供菜单、输入与确认界面。普通管理员不获得接管能力，最高管理员也不会因仅打开页面自动获得网盘写权限。没有真实部署或外部消息。
