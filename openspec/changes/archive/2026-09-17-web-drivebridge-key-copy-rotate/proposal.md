## Why

用户要求将 Key 标识复制替换为实际 Key 的复制、二次复制及轮转，并统一各页下拉框与输入框样式。

## What Changes

- Key 目录显示复制、查看和轮转动作，复制反馈可再次复制；详情中的 ID 保持只读元数据。
- 密钥面板默认遮罩，关闭后清除；轮转先显示旧 Key 和其他会话失效的影响。
- 统一绿色控件的边框、圆角、箭头、聚焦与禁用状态，优化上下文栏及窄屏 Key 卡片。

## Capabilities

### Modified Capabilities
- `drivebridge-personal-web`: 本人 Key 复制与轮转交互。

## Impact

split-owner：前端消费 backend-server/drivebridge 的新增接口；不持久化明文、不决定密钥权限。既有文件预览、分享、Agent 令牌与团队授权保持兼容。没有新增 UI 库。
