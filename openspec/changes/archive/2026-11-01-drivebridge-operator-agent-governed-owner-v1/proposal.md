# DriveBridge：治理开启后，运维签发 Agent 令牌按成员关系找属主

## Why

2026-10-31 为 Yeisme 控制中心签发只读 Agent 令牌时，`drivebridge-server agents create` 返回 `forbidden`。

- 治理开启后，空间的 `owner` 是**数据属主**（data owner）。只有治理启用时已存在的账号，其数据属主才等于用户 id；之后加入的账号由成员关系（membership）持有一个独立的数据属主。
- 运维命令按旧规则把空间的 `owner` 当成用户 id 传给签发逻辑（`Principal{UserID: sp.Owner}`）。治理检查找不到这个“用户”，拒绝签发。
- 即使签发成功，令牌也会以这个不存在的用户身份认证，在 `bindContext` 处被判 `unauthorized`。

## What Changes

- 治理开启时，`CreateAgentOperator` 在空间所属租户里查找**持有该数据属主的有效成员关系**，以该成员的用户身份（含租户与数据属主）签发；令牌归属这个用户，之后的请求与该用户自己的请求得到相同的身份。
- 找不到有效成员关系时返回 `forbidden`，不签发。
- 治理未开启时行为不变。

## Impact

- `server/internal/service/agents_operator.go`；测试 `server/internal/service/agents_operator_test.go`。
- 不改 HTTP 面、令牌格式、数据库结构；不运行 race；部署另行确认。
