## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；治理下的运维签发；`CreateAgentOperator` 在治理开启时按空间租户与数据属主查找有效成员关系，以该成员身份签发，查不到返回 forbidden；治理未开启保持原逻辑；验证 go test ./internal/service -run TestCreateAgentOperator -count=1，期望新增用例覆盖“数据属主≠用户 id 的空间可签发、令牌在治理下能读运行记录、无人持有的数据属主被拒”；变异检查（去掉治理分支后新用例以 forbidden 失败）。 | evidence: 两个用例通过；变异检查复现 forbidden；go test ./internal/service/ 全量通过（441s）；go vet 通过；未运行 race/真实账号/部署。
- [ ] 1.2 owner=cli/drivebridge；module=server；部署与真实签发；经用户确认后，用新二进制在停服窗口内为目标空间签发只读 Agent 令牌并恢复服务；期望令牌能读 /healthz、/api/v1/capabilities、/api/v1/operations。
