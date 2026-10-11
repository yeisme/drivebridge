## Why

backend 已落 key-only 部署（`dbu_` 用户密钥、`POST /auth/key` 换发会话、users 管理 HTTP 面），但 client 只会 OIDC 登录——key-only 部署里人类用户进不了浏览器，管理员也没有签发/吊销的图形面。

## What Changes

- 登录页双路径：`GET /auth/mode` 如实探测；OIDC 可用时保留账号登录按钮，**密钥登录始终作为输入路径提供**（任何部署都可能有 key 用户）。
- `POST /auth/key` 换发 12h cookie 会话后进入既有主界面；浏览器不存密钥（client 安全边界不变）。
- 「用户密钥」管理页（`me.admin` 才显示导航）：生命周期表（名称/用户/admin/过期/吊销）、批量签发表单（名称/天数/admin）、**明文密钥一次性展示弹窗**（复制即走，关窗即弃）、吊销（带确认）。
- e2e：backend fixture 预置固定 e2e 密钥（沿用 fixture-key 明文先例），Playwright 全链路（密钥登录/签发/一次性展示/吊销/普通用户无管理导航）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-web`：新增密钥登录路径与用户密钥管理页需求；OIDC 路径与既有页面零变化。

## Impact

client 拥有实现；backend 仅 `cmd/drivebridge-fixture` 预置 e2e 密钥（fixtures.Start 不动，单元测试零影响）。密钥不进 localStorage/日志/回执；一次性展示弹窗关闭后不可再取。
