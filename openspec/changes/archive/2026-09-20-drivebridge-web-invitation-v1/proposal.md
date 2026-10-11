## Why

邀请合同已能开通、激活、两次接受，但网页是三个按钮、假加入链接、收件箱藏在团队列表。登录后进网盘会漏掉邀请。

## What Changes

- 成员页一个「邀请加入」作曲器：搜索用户目录，按状态走直接加入 / 发邀请 / 开通账号。
- 被邀请人一张下一步卡；侧栏团队角标；开通后复制交付说明，去掉「复制加入入口」。
- 成员表显示待接受；支持取消、续期、拒绝。
- 治理页租户接受入口接到团队邀请卡。Precision Pine 令牌替换团队页裸 hex。

## Capabilities

### Modified Capabilities

- `team-ui`：邀请作曲器、收件箱、交付说明、角标。

## Impact

`split-owner`：只改网页交互与展示；拒绝/续期依赖服务端 `drivebridge-invitation-lifecycle-v1`。不发邮件、无开放链接、无自动网盘权限。
