## MODIFIED Requirements

### Requirement: Operator agent token issuance
服务端 CLI SHALL 支持操作员在私有状态目录直签 Agent 令牌：owner 从目标空间行推导，校验与令牌铸造与 HTTP 面同路；明文令牌只在签发输出出现一次（stdout 或 0600 文件二选一）；列表仅含生命周期元数数据。治理开启时，空间行的 owner 是数据属主，签发 SHALL 以在该空间租户中持有此数据属主的有效成员的用户身份进行；找不到这样的成员时 SHALL 返回 forbidden 且不签发。

#### Scenario: Owner derived from the space
- **WHEN** 操作员对已知空间执行 agents create
- **THEN** 令牌归属该空间属主，校验（名称/前缀/scopes/days）与 HTTP 面一致；未知空间返回 not_found

#### Scenario: Governed space owned by a data owner
- **WHEN** 治理开启，空间 owner 是某成员的数据属主（不等于其用户 id），操作员对该空间执行 agents create
- **THEN** 令牌归属持有该数据属主的成员用户，用它读取该空间的运行记录得到 200；数据属主无人持有时返回 forbidden

#### Scenario: One-shot token discipline
- **WHEN** 使用 --out 签发
- **THEN** 令牌写入 0600 文件且信封不携带明文；不带 --out 时 stdout 打印一次并提示即刻保存

#### Scenario: List never surfaces secrets
- **WHEN** 执行 agents list
- **THEN** 只读快照输出 id/name/space/scopes/expires/revoked，哈希与令牌永不出现
