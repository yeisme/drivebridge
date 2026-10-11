## Context

- 现有 `internal/service/storage.go` 用 aws-sdk-go-v2 构造 S3 兼容客户端：SigV4 静态凭据、`UsePathStyle: true` 硬编码、endpoint 强制 https（development 例外允许回环 http）。
- 平台存储对象键 `files.go`：`<instance>/<user>/<space>/<version>`——实例前缀命名空间已经存在，只是从未被当作租户边界固化。
- `S3Config` 通过 `seal/unseal`（实例 KEK）封存进 Connection 行；platform 与用户 S3 连接共用同一结构与寻址实现。
- 真实 COS 探测结论（2026-09-14，drivebridge-1384001538 桶，ap-guangzhou）：
  - SigV4 + region `ap-guangzhou` + 虚拟主机寻址（`<bucket>.cos.<region>.myqcloud.com`）：HeadBucket/Put/Get/Head/Delete 全绿，回读字节一致。
  - path-style（`cos.<region>.myqcloud.com/<bucket>/...`）：桶级拒绝，`PathStyleDomainForbidden`；且 HEAD 类请求无错误体，SDK 层退化为裸 `Forbidden`，难排障。
  - SDK 自定义 BaseEndpoint + 非路径寻址时会把 bucket 再拼一层子域；正确组合是 endpoint 写区域主机（`https://cos.ap-guangzhou.myqcloud.com`）+ bucket 字段独立声明。

## Goals / Non-Goals

交付：虚拟主机寻址声明、共享桶租户命名空间合同、隔离验证探针、共享桶部署配方文档、真实 COS 验收证据。不交付：云子凭据签发服务、密钥托管、桶级清单对账、COS 专有 API（版本/生命周期等）。

## Decisions

1. **寻址声明用 `VirtualHosted bool`（JSON `virtual_hosted`）而非翻转默认值**。零值等于今天的 path-style：既有封存连接反序列化零迁移、fixture（本地假 S3 需 path-style）零改动、无行为漂移。COS 配置显式 `"virtual_hosted": true`。
2. **共享桶隔离等级诚实化**。共享同一 root 凭据的部署在存储层彼此可见（同一凭据无服务端作用域）。两条缓解线都已有基础：云侧前缀作用域子凭据（操作员在腾讯云 CAM 签发，资源形如 `qcs::cos:<region>:uid/<appid>:<bucket>/<prefix>/*`）；应用侧空间信封加密（content-encryption-v1，root 凭据持有者无 KEK 读不了密文）。spec 只承诺命名空间隔离 + 诚实报告，不承诺字节级隔离。
3. **隔离探针放 `platform-connect`，不进 doctor**。探针本质是写操作，违反 doctor 只读 preflight 合同。探针两个落点全部在保留命名空间：桶根 `_isolation-probe/<uuid>`（区分桶根作用域）与前缀下兄弟形 `<prefix>/<uuid>/<uuid>`（区分前缀内多部署作用域）；两个都期待被拒。意外成功立即删除该对象并记 `over_scoped`；两探针均被拒记 `isolated`。不触碰任何真实租户命名空间。
4. **探针经可选接口暴露**：`type isolationProber interface { ProbeIsolation(ctx) (isolated bool, err error) }` 由 `*s3Store` 实现，CLI 类型断言调用。测试 fake Storage 不需要补方法（接口扩方法的编译级联坑）。
5. **错误可定位性**：COS HEAD 请求错误体缺失属提供方行为，不在 SDK 层二次猜测；部署文档记录该现象与 `PathStyleDomainForbidden` 对照表。

## Risks / Trade-offs

- 虚拟主机寻址要求 DNS 解析 `<bucket>.cos.<region>`；`safeDial` 已限制非全局单播地址，行为一致。
- 探针在超域凭据下会短暂写入保留命名空间再删除——接受：只在"已经超域"这一故障态发生，且立即清理；换来的能力是操作员一条命令验证凭据作用域。
- COS ListObjectsV2（Inventory）虚拟主机寻址下行为应一致；真实 COS 验收覆盖一次列表调用。

## Migration Plan

无数据迁移：新字段零值兼容旧封存配置；无在线面变化；`platform-connect` 输入新增可选键。部署文档更新为新部署配方。
