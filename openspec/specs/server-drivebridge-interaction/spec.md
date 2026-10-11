# drivebridge-interaction Specification

## Purpose
TBD - created by archiving change drivebridge-handoff-interaction-v1. Update Purpose after archive.
## Requirements
### Requirement: 交互保留原操作与授权边界

实现 SHALL 保持现有标识、权限检查和有效本地请求，并提供以下增量交互。

为 input.bind_reference 增加可选 binding_id、consumer_request_ref 和 RFC3339 request_expires_at，保留旧必填项及返回字段。增加 input.handoff_status/renew/cancel 与 handoff status/renew/cancel CLI，复用既有 service/SDK。统一身份的状态查询属于只读；续期、取消及字节获取保留原准入。

#### Scenario: 先发现后操作
- **WHEN** 客户端查询已配置目标或交接动作
- **THEN** 获得安全 ID、精确参数与配置状态，不获得新权限，不把 not_probed 宣称为 ready

#### Scenario: 恢复原操作
- **WHEN** 客户端处理已有失败或中断操作
- **THEN** 通过对应 owner 的原标识查询与恢复，不创建替代生成，不复活已终结输入

