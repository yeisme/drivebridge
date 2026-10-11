## ADDED Requirements

### Requirement: S3-compatible virtual-hosted addressing
平台存储与 S3 连接 SHALL 支持显式虚拟主机寻址，以兼容拒绝 path-style 的 S3 兼容提供方（如腾讯云 COS）；未声明时保持既有 path-style 行为，已封存连接零迁移。

#### Scenario: Virtual-hosted platform storage on COS
- **WHEN** 平台存储以 `virtual_hosted` 声明配置在拒绝 path-style 的提供方
- **THEN** 探测、写入、读取、校验与删除经 `<bucket>` 子域主机完成，失败不泄露凭据或签名材料

#### Scenario: Legacy sealed connection
- **WHEN** 打开不带 `virtual_hosted` 声明的既有封存连接
- **THEN** 仍按 path-style 寻址，行为与本变更前一致

### Requirement: Shared-bucket tenant namespace
共享桶多部署 SHALL 保持每部署独立对象命名空间：平台存储对象键恒以部署实例标识开头；该隔离是命名空间级，字节级访问控制依赖前缀作用域子凭据或空间信封加密，系统不得声称更强的隔离。

#### Scenario: Instance-scoped object keys
- **WHEN** 文件经平台存储写入共享桶
- **THEN** 对象键以本部署实例标识开头，不含其他部署命名空间

#### Scenario: Honest trust boundary
- **WHEN** 多个部署共享同一存储桶或存储凭据
- **THEN** 文档与验证面如实说明存储层无字节级隔离，并指向云侧前缀作用域子凭据与应用侧信封加密两条缓解路径

### Requirement: Storage isolation verification probe
`platform-connect` SHALL 提供隔离验证探针：仅尝试在保留探针命名空间写入自身命名空间之外的对象并期待拒绝；意外成功须立即清理并在结果中报告凭据超域；探针不得触碰真实租户命名空间。

#### Scenario: Prefix-scoped credential
- **WHEN** 凭据被云侧策略收窄到本部署命名空间
- **THEN** 两处探针写入均被拒绝，报告 isolated

#### Scenario: Over-scoped credential
- **WHEN** 凭据可写桶内任意位置或前缀下兄弟命名空间
- **THEN** 探针成功后立即删除该临时对象并报告超域，不以成功冒充隔离

#### Scenario: Probe containment
- **WHEN** 探针执行
- **THEN** 写入仅发生在保留探针命名空间，且所有成功写入都被删除
