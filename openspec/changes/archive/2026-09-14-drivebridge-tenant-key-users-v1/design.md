## Context

- 现有认证两路：浏览器会话（cookie+CSRF，OIDC 登录建立）与 Agent Bearer（`db_`+48hex，SHA-256 哈希存储，空间/前缀/作用域受限，最长 30 天）。
- `serve --enabled` 无条件 `ConfigureLogin`，issuer 缺失即退出——key-only 部署被挡在门外。
- User 表以 (Issuer, Subject) 唯一索引承载 OIDC 身份；Agent 的令牌纪律（哈希存储、一次性输出、可吊销）是现成范式。

## Goals / Non-Goals

交付：操作员 CLI 批量签发/导入用户密钥、key-only serve、admin 密钥 HTTP 批量建用户、新租户部署配方文档。不交付：OAuth/OIDC 自助注册、密钥托管/找回（丢失只能吊销重发）、用户级配额差异、Web 管理界面。

## Decisions

1. **用户密钥独立前缀 `dbu_`+48hex**，与 Agent `db_` 前缀区分，`authenticate` 按前缀路由查 `UserKey` 表——两表哈希唯一索引互不干扰，不猜表。
2. **User 身份承载复用 User 表**：Issuer="key"、Subject=用户 ID，同一套 `allow/space/file` 归属校验对 key 用户与 OIDC 用户一视同仁；不建平行身份模型。
3. **有效期默认不过期**：用户密钥是身份不是委托（Agent 才有 30 天上限）；days 可选指定，吊销是唯一即时 Kill switch。
4. **明文一次性输出**：CLI 与 HTTP 创建的响应都只在签发时刻返回明文 key；`users list` 不含哈希与明文。操作员自备 key 导入时校验 `dbu_[0-9a-f]{48}` 格式与哈希唯一。
5. **admin 是密钥属性不是用户角色**：`UserKey.Admin bool`；HTTP `POST /api/v1/users` 仅认 `p.Key.Admin`——Agent token、普通用户密钥、cookie 会话全部拒绝（fail-closed，不给 OIDC 用户开管理面）。
6. **key-only serve**：issuer 为空即跳过 `ConfigureLogin`（登录端点本就返回 `oidc_not_configured`）；origin 仍必需（CSRF 与访问流 URL 依赖它）。

## Risks / Trade-offs

- 长期不过期的身份密钥提升单 key 泄露影响面——靠吊销命令 + 部署文档"泄露即吊销重发"纪律兜住；后续波可加轮换。
- `dbu_` 前缀路由意味着用户密钥不能与 Agent 令牌格式混用——导入格式校验强制保证。

## Migration Plan

新表 `UserKey` 经现有 AutoMigrate 落库，零数据迁移；serve 行为变化仅发生在 issuer 为空的部署（原先起不来，现在能起）。
