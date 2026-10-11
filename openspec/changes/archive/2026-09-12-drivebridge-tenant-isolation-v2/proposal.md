## Why

产品需要明确多租户定位：原根合同"不自行建设组织租户平台"升级为**部署即租户**——每租户一个部署实例，单 issuer 单实例模型不变，不做组织管理/计费/跨实例共享。本 change 固化租户边界语义与升级路径，避免未来实现波在数据模型上走弯路。

## What Changes（设计，实现归后续波）

- 租户身份=部署实例：`instance.id` 即租户标识（远程引用 `drivebridge://<instance-id>/file/...` 已携带）；新增轻量 tenant profile（display name、origin、联系方式）元数据与只读查询面。
- 跨租户隔离=部署隔离：不同实例天然互不可见；跨部署访问唯一路径=远程模式限定 Agent token（现状合同）。
- 多租户升级路径=按实例分片部署（每租户独立 state/origin/issuer），明确不承诺单实例内租户化数据迁移。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增 Deployment tenant boundary 需求。

## Impact

实现波落地面很小（tenant profile 读写 + `GET /api/v1/tenant` 只读 + serve 启动参数）；身份、空间、连接模型零变化。组织管理后台、租户间共享、计费、配额分级全部继续延期。
