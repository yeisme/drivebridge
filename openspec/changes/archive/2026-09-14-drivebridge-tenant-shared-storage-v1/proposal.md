## Why

多租户已定为"部署即租户、按实例分片"。分片的成本大头是每租户一套对象存储；让多个部署共享同一桶与派生凭据是分片路线可规模化的一步。真实腾讯云 COS 对接探测（temp/integration-test-runs/cosprobe-*）证明：现有 S3 兼容层在 COS 上不可用——桶拒绝 path-style 寻址（PathStyleDomainForbidden），且 HeadBucket 无响应体时错误退化为裸 Forbidden，操作员无从定位。

本 change 补齐两件事：S3 兼容存储的虚拟主机寻址，以及共享桶场景下"租户命名空间隔离"的合同、验证探针与部署配方，使共享 key 的信任边界诚实可见。

## What Changes

- `S3Config` 新增 `virtual_hosted` 寻址声明：零值保持既有 path-style 行为，已封存连接零迁移；COS 等拒绝 path-style 的提供方显式声明。
- 固化共享桶租户不变量：平台存储对象键恒以部署实例标识开头的命名空间；隔离等级与缓解手段（前缀作用域子凭据、空间信封加密）如实入 spec。
- `platform-connect --verify-isolation`：只影响保留探针命名空间的写探针，判定凭据是否被云侧策略收窄到本部署命名空间；意外成功即清理并报告超域。
- 部署文档新增共享桶多部署配方：云控制台签发前缀作用域子凭据的策略样例、platform-connect 输入样例、验证流程与加密兜底说明。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增虚拟主机寻址、共享桶租户命名空间、隔离验证探针三个需求。

## Impact

backend-server/drivebridge 拥有实现与验收；mcp/client 零变化。不改变"部署即租户"边界：共享的是哑对象存储桶与存储凭据，不是内容或授权面；平台不代签发云子凭据，签发权留在云控制台。真实 COS 验收依赖操作员提供的一次性凭据，凭据只经环境变量进入测试，不落库不入库不提交。
