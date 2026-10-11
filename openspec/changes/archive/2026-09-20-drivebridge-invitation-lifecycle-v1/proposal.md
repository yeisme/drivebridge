## Why

团队邀请已能绑定用户并接受，但生命周期不完整：被邀请人不能拒绝，过期后只能取消再发，列表缺少姓名和下一步，网页无法编排成一张「下一步」卡。

## What Changes

- 新增 `team-invite-decline`：仅绑定用户可将 pending 未过期邀请标为 `declined`，不创建成员。
- 新增 `team-invite-resend`：管理员将同一 invitation id 的 pending（含已过期）续七天。
- 邀请读取投影增加可选 `user_name`、`inviter_name`、`membership_status`、`next_step`。
- 不发邮件、不开放链接、不自动授网盘权限、不改表结构。

## Capabilities

### New Capabilities

- `team-invite-decline` / `team-invite-resend`：邀请拒绝与续期。

### Modified Capabilities

- `team-workspace`：邀请投影与状态 `declined`。

## Impact

`split-owner`：状态在本服务。网页消费加法字段。旧客户端可忽略新字段。回滚停用新动作即可。
