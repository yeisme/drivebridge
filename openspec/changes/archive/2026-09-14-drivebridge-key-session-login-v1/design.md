## Context

- 会话铸造：OIDC 回调 `Session{Hash, Owner, CSRF, Expires=12h}` + cookie；`authenticate` 会话路返回 `Principal{UserID, CSRF}`。
- 用户密钥：`dbu_`+96hex，SHA-256 存储，`UserKey.Admin` 属性；HTTP `POST /api/v1/users` 仅认 `p.Key.Admin`。
- client 边界（AGENTS）：不把登录或 Agent token 存进 localStorage——浏览器持有的只能是服务端会话 cookie。

## Goals / Non-Goals

交付：密钥换发会话登录、admin 会话继承、users list/revoke HTTP、`/auth/mode`。不交付：key 记住我/长效设备授权、登录限速（100-hex 猜测不可行，与 API Bearer 同风险面）、Web 端签发历史审计页。

## Decisions

1. **换发而非存键**：`POST /auth/key {key}` 校验 UserKey（吊销/过期同 authenticate 规则）→ 铸造同一枚 Session cookie。浏览器从不知道也不保存密钥；登出/过期即失效。与 OIDC 会话完全同构，CSRF 机制复用。
2. **Session.Admin 列继承**：admin 密钥换发的会话写 `Admin=true`；`Principal.Admin` 由 `Key.Admin` 或 `Session.Admin` 置位——管理判定统一到 Principal，`CreateUserUsers` 改认 `p.Admin`。
3. **失败不区分**：错 key/已吊销/已过期统一 unauthorized，响应体不泄露存在性。
4. **`GET /auth/mode` 无需认证**：仅报告 `{oidc: bool, key: true}`（issuer 是否配置）；不含任何租户敏感信息。
5. users 管理接口返回形状对齐 agents（data 信封、生命周期字段、无哈希）。

## Risks / Trade-offs

- admin 会话扩大了 admin 密钥泄露的影响窗口（12h 会话存续期内吊销密钥不杀会话）——v1 接受并在文档注明"吊销后应重启或等会话过期"；后续波可加会话随密钥吊销级联。
- `/auth/key` 无限速与 API Bearer 面等同，不新增风险等级。

## Migration Plan

Session 表新增列（AutoMigrate）；既有会话无该列值即 false，行为不变。
