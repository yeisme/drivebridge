# team-workspace Specification

## Purpose
TBD - created by archiving change drivebridge-team-workspace-v1. Update Purpose after archive.
## Requirements
### Requirement: 团队变更共享权限与版本规则

服务 SHALL 在新旧入口保护最后一位团队管理员和最后一位空间数据负责人，新 team 动作必须校验 expected_revision。

#### Scenario: 陈旧修改

- **WHEN** 操作者提交旧版本号
- **THEN** 返回 409，团队和权限状态不改变

#### Scenario: 旧入口移除最后负责人

- **WHEN** 旧 team-member 或 permission 请求会失去最后负责人
- **THEN** 操作被拒绝且提示交接

### Requirement: 团队存储显式分配

服务 SHALL 仅允许平台或当前管理员在当前租户的自有 S3 兼容连接分配给团队，访问时校验具体空间分配。

#### Scenario: 连接仍被网盘引用

- **WHEN** 管理员请求取消仍被使用的分配
- **THEN** 返回冲突并保留绑定和文件

### Requirement: 归档只读且可撤销

服务 SHALL 保留原成员读取权限，拒绝归档团队的新写入，并撤销分享、临时授权和 Agent 访问。

#### Scenario: 归档期间完成上传

- **WHEN** 上传在团队归档后尝试提交
- **THEN** 不发布新文件；原操作者可取消并释放预留容量

#### Scenario: 恢复团队

- **WHEN** 租户或平台管理员恢复团队
- **THEN** 原长期空间权限恢复有效，旧分享、临时授权与 Agent 不自动复活

### Requirement: 用户绑定邀请

服务 SHALL 将邀请绑定指定用户与团队，仅在租户成员关系有效且邀请未过期时接受。

#### Scenario: 他人尝试接受

- **WHEN** 非指定用户提交邀请 ID
- **THEN** 服务拒绝且不创建成员关系

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

