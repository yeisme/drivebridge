# key-self-delete Specification

## Purpose
TBD - created by archiving change drivebridge-key-delete-trash-batch-v1. Update Purpose after archive.
## Requirements
### Requirement: 本人可吊销和删除登录 Key

服务 SHALL 允许 Key 所属用户在确认后吊销或删除本人 Key。删除 SHALL 吊销、归档、擦除密文并删除该 Key 换发的会话。SHALL NOT 删除或吊销该用户最后一把有效 Key。最后一个有效管理员 Key SHALL 返回 `last_admin_required`。Agent 与服务身份 SHALL 被拒绝。

#### Scenario: 删除多余 Key
- **WHEN** 用户拥有至少两把有效 Key 并确认删除其中一把
- **THEN** 该 Key 无法再登录或 reveal，列表明 `revoked` 且 `can_copy` 为 false

#### Scenario: 拒绝删光
- **WHEN** 用户只剩一把有效 Key 并请求吊销或删除
- **THEN** 返回 `last_active_key`，该 Key 仍可用于登录

### Requirement: 本人可禁用和启用登录 Key

服务 SHALL 允许所属用户禁用有效 Key：禁用后不能登录、该 Key 换发的会话失效，但密文保留。启用 SHALL 恢复登录，不恢复旧会话。SHALL NOT 禁用最后一把有效 Key。已吊销、已归档或已过期的 Key SHALL NOT 被启用。Agent 与服务身份 SHALL 被拒绝。

#### Scenario: 禁用后无法登录，启用后恢复
- **WHEN** 用户禁用一把非最后有效 Key
- **THEN** 该 Key 的 Bearer 与会话认证失败；启用后同一明文可再次登录，且 reveal 仍可用

#### Scenario: 拒绝禁用最后一把
- **WHEN** 用户只剩一把有效 Key 并请求禁用
- **THEN** 返回 `last_active_key`

