## 1. 收件箱与角标

- [x] 1.1 下一步卡、拒绝、跨租户切换后留卡；治理页接到 `#/teams`
  - 范围：`src/Teams.tsx`、`src/Governance.tsx`、`src/teams.css`
- [x] 1.2 侧栏团队数量角标
  - 范围：`src/main.tsx`、`src/style.css`

## 2. 作曲器与交付

- [x] 2.1 「邀请加入」搜索目录、路径分流、开通交付说明、去掉复制加入入口
  - 范围：`src/Teams.tsx`、`src/i18n.ts`
- [x] 2.2 成员表待接受行；续期/取消
- [x] 2.3 `teams.css` 去裸 hex；文档 `docs/team-workspace.md`、`docs/ui/DESIGN.md`

## 3. 验证

- [x] 3.1 更新 `tests/teams.spec.ts` 走作曲器开通路径
  - 验证：`DRIVEBRIDGE_TEST_TEAM_DEMO=1 python3 scripts/check.py -- pnpm exec playwright test tests/teams.spec.ts`
  - 证据：`temp/integration-test-runs/20260920T050412Z-9a860689`
