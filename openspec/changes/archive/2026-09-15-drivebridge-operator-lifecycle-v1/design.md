# Design: drivebridge-operator-lifecycle-v1

## 决策

1. **避让并行 lane**：本波全部走新文件（status.go/agents.go/agents_operator.go）+ main.go 注册（在途 lane 未触碰），不与 `input.bind_reference`/key 格式波共享任何文件；`createAgent` 签名零改动——operator 路径直接传 `Principal{UserID: sp.Owner}`，owner 从空间行推导，避免操作员猜 user id 也避免改 auth.go。
2. **status 只读纪律**：本地模式绝不 service.Open（启动修复会动活操作），用 `mode=ro` 快照聚合 SQL 一发；远程模式按序探测 healthz（无鉴权）→ me（验 token）→ spaces，每步失败给专属码（service_unreachable/token_rejected/service_request_failed），token 文件强制 0600+单行可打印。
3. **一次性令牌纪律**：agents create 明文令牌与 users import 同律——stdout 打印一次或 `--out` 写 0600（二选一，--out 时信封不携带 token，只带 token_file 路径）；list 永不出现哈希/令牌。
4. **列名陷阱规避**：快照 SQL 直接用物理列名（spaces.used/reserved、agents.expires）而非 json 名——GORM 列名≠json 标签在本仓已第三次踩。

## 缓办

- `users verify --key`：需挂进 users 子命令树（commands.go 在途），待树清后续 change。
- `serve --events` 监督流：同因 commands.go 在途。
