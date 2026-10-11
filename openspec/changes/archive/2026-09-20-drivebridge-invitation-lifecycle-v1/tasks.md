## 1. 动作与投影

- [x] 1.1 实现 `team-invite-decline`、`team-invite-resend`，409 `invitation_not_pending`，审计通知绑定用户或邀请人
  - owner: backend-server/drivebridge
  - 范围：`internal/service/teams_change.go`、`http.go`、`error_messages.go`
- [x] 1.2 邀请列表与本人收件箱投影补姓名、`membership_status`、`next_step`
  - 范围：`internal/service/teams_read.go`
- [x] 1.3 测试：他人拒绝、过期拒绝、续期同 id、已接受不可续、不授文件权限
  - 验证：`go test ./internal/service -count=1 -run 'Invitation'`
- [x] 1.4 文档：`docs/team-workspace.md`、`docs/tenant-governance.md` 列出新动作
