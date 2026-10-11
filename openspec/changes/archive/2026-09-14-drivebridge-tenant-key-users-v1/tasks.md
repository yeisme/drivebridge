## 1. 实施与验证

- [x] 1.1 `UserKey` 模型 + AutoMigrate + `dbu_` 前缀路由进 `authenticate`（吊销/过期/哈希校验）；Principal 扩展
- [x] 1.2 操作员 CLI：`users import`（stdin JSON 批量，生成或导入，明文一次输出）、`users list`（无哈希无明文）、`users revoke`
- [x] 1.3 key-only serve：issuer 为空跳过 ConfigureLogin；登录端点如实 oidc_not_configured
- [x] 1.4 HTTP `POST /api/v1/users`：仅 admin 用户密钥可用，普通密钥/Agent/会话拒绝；与 CLI 同校验
- [x] 1.5 单测+fixture e2e：批量签发/导入格式拒绝/吊销后 unauthorized/admin 开面矩阵/key 用户全功能上传回读；`python3 scripts/check.py`、`go vet`、`CGO_ENABLED=0 go build` 全绿
- [x] 1.6 部署 runbook：新租户四步配方（config init → platform-connect → users import → serve）与 key 泄露处置

验证证据：`temp/integration-test-runs/20260914T102202Z-0757d275/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
