## ADDED Requirements

### Requirement: Operator status projection
服务端 CLI SHALL 提供只读 `status` 命令：远程模式以 0600 用户密钥文件探测运行中部署的 healthz、身份与空间并输出单一投影；本地模式以只读快照投影 state 完整性、空间数、配额合计与用户密钥计数；两种模式的人读摘要 SHALL 携带恰好一条下一步建议，且不输出凭据。

#### Scenario: Remote projection
- **WHEN** 以 --url 与 --token-file 运行 status
- **THEN** healthz 可达性、token 对应身份（含 admin）与空间列表折叠进一个信封；token 文件组/其他可读时拒绝

#### Scenario: Local snapshot
- **WHEN** 无 --url 运行 status
- **THEN** 只读快照输出 state 三件存在性、空间计数、max/used/reserved 合计与 user key 总数/存活数；未初始化的部署如实报告并指向初始化下一步

#### Scenario: Typed failure per hop
- **WHEN** healthz 不可达、token 被拒或空间投影失败
- **THEN** 分别以 service_unreachable、token_rejected、service_request_failed 失败，不冒充成功

### Requirement: Operator agent token issuance
服务端 CLI SHALL 支持操作员在私有状态目录直签 Agent 令牌：owner 从目标空间行推导，校验与令牌铸造与 HTTP 面同路；明文令牌只在签发输出出现一次（stdout 或 0600 文件二选一）；列表仅含生命周期元数数据。

#### Scenario: Owner derived from the space
- **WHEN** 操作员对已知空间执行 agents create
- **THEN** 令牌归属该空间属主，校验（名称/前缀/scopes/days）与 HTTP 面一致；未知空间返回 not_found

#### Scenario: One-shot token discipline
- **WHEN** 使用 --out 签发
- **THEN** 令牌写入 0600 文件且信封不携带明文；不带 --out 时 stdout 打印一次并提示即刻保存

#### Scenario: List never surfaces secrets
- **WHEN** 执行 agents list
- **THEN** 只读快照输出 id/name/space/scopes/expires/revoked，哈希与令牌永不出现
