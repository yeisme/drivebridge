## Why

用户需要通用文件基础设施，不自行建设组织租户平台。身份复用现成 OIDC，文件归属及 Agent 委托仍由 DriveBridge 校验。

## What Changes

React/TypeScript 网页提供文件、目录、分片进度、失败恢复、版本、存储连接、Agent 授权和回收站，消费公共 API。

## Capabilities

### New Capabilities
- `drivebridge-personal-web`：个人文件空间与授权的增量合同。

### Modified Capabilities
无已发布合同删除。

## Impact

新增入口默认关闭；不自动迁移旧本机数据或扩大旧凭据。组织、分享、计费、挂载、离线同步、多实例高可用延期。真实账号、付费 Provider、公网部署独立验收。
