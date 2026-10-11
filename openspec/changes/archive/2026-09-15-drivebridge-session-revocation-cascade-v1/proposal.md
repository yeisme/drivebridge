## Why

吊销用户密钥不会杀死它换发的浏览器会话——admin 会话在 12h 窗口内继续持有管理权，多租户共享部署的高敏场景不可接受（runbook 现只能提示"吊销后重启或等过期"）。

## What Changes

- 密钥换发会话记录来源密钥 id；认证时来源密钥已吊销/过期则会话立即失效。
- OIDC 会话与无来源会话行为不变；runbook 撤回"12h 窗口存续"告警话术。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：Key exchange session login 需求增补"吊销级联"场景。

## Impact

Session 表新增列（AutoMigrate）；每次带会话认证多一次主键查键校验，可忽略。
