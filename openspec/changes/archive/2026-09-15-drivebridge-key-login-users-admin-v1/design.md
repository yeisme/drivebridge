## Context

- client 登录：未登录视图单一 OIDC 按钮（`/auth/login`）；会话 cookie + CSRF 由 `api()` 统一携带。
- backend：`GET /auth/mode`（无需认证）报告 `{oidc, key}`；`POST /auth/key {key}` 换发 12h 会话（跨 Origin 拒绝）；`GET /api/v1/users` → `{keys: [...]}`；`POST /api/v1/users` 批量签发；`POST /api/v1/users/{id}/revoke`；`/me` 带 `admin`。
- client AGENTS：不把登录或 Agent token 存进 localStorage；必须覆盖"token 一次显示"。

## Goals / Non-Goals

交付：双路径登录、admin 用户密钥管理页、fixture e2e 预置密钥。不交付：密钥记住我/多标签会话管理、签发历史审计页、密钥轮换 UI、key-only fixture 变体。

## Decisions

1. **密钥输入恒为二级路径**：OIDC 可用时折叠为"使用密钥登录"切换；不可用时为唯一路径。任何部署都可能同时有 OIDC 用户与 key 用户，按 mode 如实呈现而不是二选一。
2. **换发即忘**：keyLogin 成功后仅重跑既有 `/me` 流程；密钥字符串不进任何 state 持久层/URL/storage。失败统一 `unauthorized` 文案。
3. **一次性展示弹窗**：签发响应的明文密钥只在模态展示（复制按钮），确认关闭后从内存清除；列表接口本就无明文，无"再次查看"。
4. **管理页按 `me.admin` 条件渲染导航**：普通用户不可见（非隐藏路由鉴权，服务端仍是权威）。
5. fixture 预置固定 e2e 密钥进 `cmd/drivebridge-fixture`（非 fixtures.Start）：`dbu_<11*48>`（普通）与 `dbu_<22*48>`（admin），沿用 fixture-key/fixture-secret 明文测试凭据先例。

## Risks / Trade-offs

- 双路径登录让暴力面从 OIDC 扩到 /auth/key——与 API Bearer 同风险等级（100-hex），接受；后端统一 unauthorized。
- e2e 固定密钥属测试凭据，仅 fixture 本机回环有效。

## Migration Plan

纯增量：新导航项、新视图、新 API 封装；不动既有页面与路由。
