## Why

新租户四步在 ④ serve 之后断链：没有一条命令回答「部署到底活没活、身份能不能用、配额还剩多少」；key-only 部署签发 Agent 令牌必须走 session+CSRF 的 HTTP 往返；四步文档照抄会丢一次性 key（--out 未被推荐）；配额余量口径（max/used/reserved）无文档。

## What Changes

- `drivebridge-server status`：远程模式（`--url --token-file`）探测 healthz + `/me` 身份 + `/spaces` 单信封投影，token 文件 0600 强制；本地模式只读快照投影 state 三件/空间数/配额三值合计/user key 计数/平台存储状态；两种模式人读摘要都带恰好一条 Recommended next step。
- `drivebridge-server agents create/list`：operator 在私有状态目录直签 Agent 令牌——owner 从目标空间行推导（操作员不猜 user id），校验与令牌铸造与 HTTP 面完全同路（`createAgent` 原样复用）；明文令牌只出现一次（stdout 或 `--out` 0600 文件二选一）；list 只读快照、仅生命周期元数据。
- runbook ③ 推荐直接 `users import --out dist.csv`；新增 ⑤ 启动后 `status` 核对；docs 补 agents 直发与配额口径（余量 = max − used − reserved，预留即扣减）。

## Capabilities

### New Capabilities

无（挂在既有 owner 能力下新增需求）。

### Modified Capabilities

- `drivebridge-personal-service`：ADDED 需求 Operator status projection、Operator agent token issuance。

## Impact

backend-server/drivebridge；不动 HTTP/MCP 既有契约（status 走既有端点，agents 复用既有 createAgent 语义）；users verify 与 serve --events 留待工作树清空后的后续 change。
