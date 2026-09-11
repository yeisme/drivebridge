## Why

用户需要通用文件基础设施，不自行建设组织租户平台。身份复用现成 OIDC，文件归属及 Agent 委托仍由 DriveBridge 校验。

## What Changes

旧本机模式保留。新增 Go SDK 远程模式，限定 token 与本机目录，HTTP 传字节，MCP 传元数据与稳定引用。

## Capabilities

### New Capabilities
- `drivebridge-remote-client`：个人文件空间与授权的增量合同。

### Modified Capabilities
无已发布合同删除。

## Impact

新增入口默认关闭；不自动迁移旧本机数据或扩大旧凭据。组织、分享、计费、挂载、离线同步、多实例高可用延期。真实账号、付费 Provider、公网部署独立验收。
