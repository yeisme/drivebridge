# drivebridge-cloud-connectors Specification

## Purpose
TBD - created by archiving change drivebridge-cloud-connectors-v2. Update Purpose after archive.
## Requirements
### Requirement: 三类云端能力矩阵
系统 MUST 独立申报OneDrive/S3/Google Drive的浏览、传输、历史版本、复制、软删除与增量能力。

#### Scenario: 用户连接Google Drive
- **WHEN** 完成授权并绑定目录
- **THEN** 服务端可分页列举允许根；本机未在线不阻止云端读取。

#### Scenario: 后端不支持版本读取
- **WHEN** 消费旧revision
- **THEN** 明确 version_unavailable，不返回最新版。

### Requirement: 外部观测与受管版本
系统 MUST 区分ExternalEntry与受管File/Version，只有核验后的导入才能承诺受管副本。

#### Scenario: 外部文件导入
- **WHEN** 下载固定观测并计算摘要
- **THEN** 返回受管固定引用及来源关联。

#### Scenario: 读取期间源变化
- **WHEN** 完成导入校验
- **THEN** source_changed，旧观测不能冒充已保存版本。

### Requirement: 范围和游标失效
系统 MUST 将扫描与游标限制在当前连接根，不把不完整枚举作为删除依据。

#### Scenario: 分页完成
- **WHEN** 续读同一根游标
- **THEN** 返回有界页与coverage。

#### Scenario: 游标过期或权限缩小
- **WHEN** 续扫
- **THEN** 要求重建基线或拒绝，不扩大为全盘扫描。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。

