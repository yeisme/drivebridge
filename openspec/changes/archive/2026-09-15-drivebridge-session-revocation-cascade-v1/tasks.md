## 1. 实施与验证

- [x] 1.1 Session.OriginKey 记录来源密钥；authenticate 会话路来源键已吊销/过期即 unauthorized；OIDC/无来源会话不变
- [x] 1.2 单测：换发会话在 revoke/rotate 后 401、OIDC 会话不受影响；check.py/vet/build 全绿
- [x] 1.3 runbook 撤回 12h 窗口告警、改为吊销即失效

验证证据：`temp/integration-test-runs/20260915T162047Z-1468316c/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
