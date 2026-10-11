## ADDED Requirements

### Requirement: 被邀请人可拒绝绑定邀请
服务 SHALL 允许邀请绑定用户对未过期 pending 邀请执行 `team-invite-decline`，将状态改为 `declined` 且不创建团队成员。非绑定用户 SHALL 被拒绝。

#### Scenario: 本人拒绝
- **WHEN** 绑定用户提交 `team-invite-decline` 且邀请 pending 未过期
- **THEN** 状态为 `declined`，不出现团队成员行

#### Scenario: 他人拒绝
- **WHEN** 非绑定用户提交同一邀请 ID
- **THEN** 服务拒绝且邀请保持 pending

### Requirement: 管理员可续期同一邀请
服务 SHALL 允许有邀请权限的管理员对 pending 邀请（含已过期）执行 `team-invite-resend`，将到期时间设为现在起七天，保留同一 invitation id。已接受、已取消或已拒绝 SHALL 返回 `invitation_not_pending`。

#### Scenario: 过期后续期
- **WHEN** 管理员对已过期但仍为 pending 的邀请续期
- **THEN** 同一 id 变为未过期 pending，不新建行

#### Scenario: 已接受不可续
- **WHEN** 管理员对已接受邀请续期
- **THEN** 返回 `invitation_not_pending` 且到期时间不变

### Requirement: 邀请投影包含下一步
`GET /api/v1/governance/team-invitations` 与团队邀请列表 SHALL 返回可选 `user_name`、`inviter_name`、`membership_status`；本人收件箱另含 `next_step`（`switch_tenant`、`accept_tenant`、`accept_team`、`expired`）。SHALL NOT 把过期行在库中改成 `expired`。

#### Scenario: 跨租户收件箱
- **WHEN** 当前租户与邀请租户不同且邀请未过期
- **THEN** `next_step` 为 `switch_tenant`
