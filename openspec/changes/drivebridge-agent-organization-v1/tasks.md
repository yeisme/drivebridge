## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与风险分级与不可自批；验证 openspec validate drivebridge-agent-organization-v1 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T235701Z-a482d228）
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。（接口表 organize_plan/get/人类confirm/organize_apply/organize_compensation_plan/organization-policy 与错误码 approval_required/plan_stale/policy_revoked/compensation_conflict 已复核一致；动作词表 add_label/move/trash/remove_label/restore，覆盖与永久删除不进Agent整理面）

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPAgentOrganization模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260926T000040Z-6c21787d；organization_policies/plans/items 纯新增表+幂等键；PlanIdentityAndApprovalGate 身份/重放/审批门）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260926T000040Z-6c21787d；纯新增表；标签/文件既有表只加行不改列）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现风险分级与不可自批和计划冻结与再校验的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260926T000040Z-6c21787d；ExecutePolicyAndDrift：确认后执行 rule 标签落地可查、补偿计划派生且不立即执行、policy_scoped 免确认路由）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点部分成功和补偿；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPAgentOrganization -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260926T000040Z-6c21787d；源漂移单项 stale 其余保留、policy_revoked 停止未执行项、错误摘要 impact_digest 绑定确认、Agent 禁确认/禁策略）

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260926T000040Z-6c21787d；HTTP/SDK/MCP 同一应用服务；organize_plan/get/apply/compensation 四工具可发现，confirm/policy 无Agent工具）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“精确查询→虚拟整理建议→规则授权→逐项执行→补偿预览”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPAgentOrganization -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260926T000040Z-6c21787d；fixture 流：上传→计划→人类确认→执行→标签可查→补偿派生，全程公开接口）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPAgentOrganization -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260926T000040Z-6c21787d；spec场景：计划冻结与审批分离/逐项复检与stale/预授权与撤销/policy_scoped非免鉴权）
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；百万目录生成10000项计划，分页200；20并发计划/规则变更与权限撤销，断言无越界修改、部分恢复不重复，补偿保留后续版本。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。（run 20260926T000040Z-6c21787d；10000项计划与20并发规则变更规模 fixture 本机未跑，按 exclusions 留白记录）

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-agent-organization-v1 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260926T000040Z-6c21787d 后复跑通过；git diff --check 干净；docs/file-platform/query-organization.md Agent整理节新增实施状态+留白说明）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260926T000151Z-db2065ff：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict(20260926T000129Z-e98023b4)/git diff --check 全过；规模与部署保持独立验收未完成）
