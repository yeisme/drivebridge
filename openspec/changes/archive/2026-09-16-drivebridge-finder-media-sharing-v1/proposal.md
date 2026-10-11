## Why

文件浏览缺少真实目录树、统一媒体卡片与固定版本预览，传输、分享及创作引用不能形成连续操作体验。

## What Changes

实现 Finder 风格文件界面，接入分页目录、统一预览、传输结果小图、分享、Key 归档和素材创作面板；本机入口复用同一界面与 owner 通道。

## Capabilities

### New Capabilities
- `finder-media-sharing`: 文件浏览、媒体预览、分享与素材引用。

## Impact

Web owner 为 client/drivebridge-client；目录、权限、分享、预览与生成交接分别消费现有 owner 的增量合同。用户已确认设计并授权本地实施。线上激活单独处理。
