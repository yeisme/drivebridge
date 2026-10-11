## ADDED Requirements

### Requirement: 内容owner边界
系统 MUST 将理解与语义检索交给Manju及领域producer，仅保存固定版本关联与安全状态。

#### Scenario: 用户提交视频检索
- **WHEN** Manju已有可读Anatomia证据
- **THEN** 返回原时间段和truth level。

#### Scenario: 缺音频producer合同
- **WHEN** 用户检索语音内容
- **THEN** unsupported并保留元数据搜索，不伪造摘要。

### Requirement: 空间策略与独立处理
系统 MUST 默认关闭自动理解，已批准范围/预算/外发策略才允许产生处理意图。

#### Scenario: 用户启用指定目录策略
- **WHEN** 新版本提交
- **THEN** 形成独立原任务引用并在预算内处理。

#### Scenario: 只执行搜索或预算耗尽
- **WHEN** 没有现成索引
- **THEN** 返回缺失原因，不发起付费模型。

### Requirement: 版本和双边授权
系统 MUST 校验源文件与Manju项目双边权限，旧版本结果不得覆盖新字节。

#### Scenario: 固定旧版本仍获授权
- **WHEN** 回读证据
- **THEN** locator和摘要属于旧版本并明确标记。

#### Scenario: 删除后处理完成
- **WHEN** worker回写关联
- **THEN** 拒绝激活并请求Manju清理派生索引。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
