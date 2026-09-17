# drivebridge-interaction Specification

## Purpose
TBD - created by archiving change drivebridge-relay-discovery-v1. Update Purpose after archive.
## Requirements
### Requirement: 交互保留原操作与授权边界

实现 SHALL 保持现有标识、权限检查和有效本地请求，并提供以下增量交互。

新增只读 relay_targets，使用现有 RelayTarget 安全投影。返回 ID、源空间、项目、disabled 和 not_probed；隐藏端点和凭据文件路径。CLI 明确必要配置选项，原 targets 数组输出不变。

#### Scenario: 先发现后操作
- **WHEN** 客户端查询已配置目标或交接动作
- **THEN** 获得安全 ID、精确参数与配置状态，不获得新权限，不把 not_probed 宣称为 ready

#### Scenario: 恢复原操作
- **WHEN** 客户端处理已有失败或中断操作
- **THEN** 通过对应 owner 的原标识查询与恢复，不创建替代生成，不复活已终结输入

