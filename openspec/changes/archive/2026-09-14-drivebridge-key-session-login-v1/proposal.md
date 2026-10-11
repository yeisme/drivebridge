## Why

key-only 部署与 key 用户已落地，但网页端只会 OIDC 登录——key-only 部署里人类用户进不了浏览器；用户管理面只有创建接口，list/revoke 仅 CLI，浏览器侧无管理 UX。前后端在此断链。

## What Changes

- `POST /auth/key`：粘贴一次用户密钥换发与 OIDC 同一枚 cookie 会话（12h，admin 属性继承）；浏览器不存储任何 token，登出即弃。
- Session 增 admin 继承：admin 密钥换发的会话即 admin 会话；`Principal.Admin` 统一承载（密钥 Bearer 或 admin 会话均可）。
- HTTP 管理面对齐 agents 范式：`GET /api/v1/users`（生命周期元数据，无哈希无明文）与 `POST /api/v1/users/{id}/revoke`，仅 admin。
- `GET /auth/mode`：如实报告本部署登录方式（oidc/key），供前端选择登录界面。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增密钥换发会话登录、用户管理 HTTP 面、登录方式如实报告三个需求；admin 判定从"仅密钥属性"升级为"密钥属性或其换发会话"。

## Impact

backend-server 拥有实现；client 消费 `/auth/key` 与 users 管理接口（后续 client change）；mcp 零变化。会话安全语义不变（cookie+CSRF+过期）；密钥仍只在签发输出出现一次，登录是单向换发不可逆。
