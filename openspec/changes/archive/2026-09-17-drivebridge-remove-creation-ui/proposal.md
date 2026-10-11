## Why

用户明确要求网盘不再出现创作功能。现有“用于创作”入口将提示词、模型连接和生成任务带入文件管理页面，超出当前产品边界。

## What Changes

- 删除共享 Finder 的创作入口及服务端、本机页面中的创作面板、状态与专用样式。
- 保留文件管理、上传、媒体预览、分享和固定引用复制。
- 更新当前规范与使用说明，验证移除入口后完整文件流程仍可使用。

## Capabilities

### Modified Capabilities
- `finder-media-sharing`: 网盘仅承载文件浏览、预览、分享与引用，不提供生成交互。

## Impact

归属为 fit：仅修改 client/drivebridge-client。既有后端 API、外部创作服务和持久化数据保持兼容，不迁移或删除用户数据。
