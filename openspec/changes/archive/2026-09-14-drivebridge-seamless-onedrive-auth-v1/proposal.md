## Why

首用最大摩擦在认证与身份发现：`auth` 打开 rclone 交互配置器（多步英文向导），drive-id 要从 rclone.conf 手抄，root-id 必须用 Graph Explorer 手查 item ID（activation-local.md 第 1-3 步）。个人单独使用场景下这些步骤应折叠为一条命令。

## What Changes（设计，实现归后续波）

`auth` 默认改为一站式 device-code 流：复用 rclone 内置公共 onedrive client id，打印 verification URI/code 由用户在浏览器完成登录；成功后自动发现默认 drive-id（`GET /me/drive`）并提供 root 交互选择（`GET /me/drive/root/children` 编号菜单）。旧 rclone 交互配置器保留为 `auth --rclone-interactive`。全程不打印 token。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-local-files`：新增 Seamless first-use onboarding 需求（设备码流免注册应用、drive/root 自动发现、凭据不进输出）。

## Impact

仅 CLI 认证/配置面；MCP 工具面、传输合同、空间身份模型不变。space-add 增 `--auto`/`--root-name` 非等价交互路径。doctor 增可选 `--probe-auth`（默认仍 not_probed，保持诚实就绪）。实现波落地时重写 activation-local.md 第 1-3 步为单命令路径；不迁移已有 rclone 配置。
