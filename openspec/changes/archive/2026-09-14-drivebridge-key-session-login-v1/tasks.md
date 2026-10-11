## 1. 实现与验证

- [x] 1.1 `POST /auth/key` 换发会话 + Session.Admin 继承 + `Principal.Admin` 统一（Key.Admin/Session.Admin 两路置位；CreateUserUsers 改认 p.Admin）
- [x] 1.2 users 管理 HTTP 面：`GET /api/v1/users`、`POST /api/v1/users/{id}/revoke`（仅 admin；元数据无哈希）
- [x] 1.3 `GET /auth/mode` 如实报告 oidc/key
- [x] 1.4 单测+e2e：换发/坏 key/吊销后换发拒绝/admin 会话触达与普通会话拒绝/list 无哈希/HTTP 吊销生效/mode 两形态；`python3 scripts/check.py`、`go vet`、`CGO_ENABLED=0 go build` 全绿
- [x] 1.5 部署 runbook：key-only 浏览器登录说明 + admin 会话 12h 窗口与吊销语义注记

验证证据：`temp/integration-test-runs/20260914T160617Z-b3813862/summary.json`（相对项目根目录，go test -race ./... 全绿）。CLI 侧证据 `temp/integration-test-runs/20260914T160951Z-00b34c32`。
