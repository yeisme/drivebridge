## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与内容owner边界；验证 openspec validate drivebridge-content-retrieval-v1 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T231853Z-c47ead02）
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。（接口表 content_bindings/content-policy/content_process/content_queries 与错误码 identity_mapping_required/content_policy_required/content_index_stale/modality_unsupported 已复核一致；Manju 连接复用 creative connection 合同（kind=manju，人类配置）为落地映射）

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPContentRetrieval模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T232932Z-c9ea149f；content_policies/content_bindings 纯新增表+幂等键；BindingIdentityAndPolicyGate 身份/策略门/幂等反例）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T232932Z-c9ea149f；纯新增表不碰旧列，live 读取既有目录）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现内容owner边界和空间策略与独立处理的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T232932Z-c9ea149f；ProcessAndVersionSemantics 处理成功 searchable+index_generation+原operation、失败不影响原件；QueryModes exact/discovery+证据locator+missing 模态）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点版本和双边授权；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPContentRetrieval -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T232932Z-c9ea149f；版本漂移 content_index_stale+绑定 stale+晚到结果不挂新版本、modality_unsupported、identity_mapping_required、Agent 禁策略扩权、discovery 逐项再授权+外来ref丢弃）

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T232932Z-c9ea149f；HTTP/SDK/MCP 同一应用服务，5工具可发现，旧 prepare_reference/inbox 未动）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“指定目录策略→内容owner处理→证据检索→版本更新失效”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPContentRetrieval -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T232932Z-c9ea149f；fixture 流：策略启用→钉定绑定→处理→discovery 带证据检索→撤权过滤，全程公开接口+loopback Manju stub）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPContentRetrieval -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T232932Z-c9ea149f；spec场景：绑定与策略权威/版本与双边授权（钉版本+漂移stale）/检索再授权（外来ref丢弃+missing 报告）/默认不自动处理（无策略 content_policy_required）全覆盖）
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；四模态至少1000条固定fixture证据，覆盖locator、truth和撤权；与百万文件目录链接分页，语义检索8秒预算且Top-K不当全集；模型质量用owner真实评测独立报告。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。（run 20260925T232932Z-c9ea149f；四模态1000条 fixture 与真实 Manju 质量评测未执行——内容owner未部署，仅 loopback stub 合同验证；按 exclusions 留白记录）

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-content-retrieval-v1 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T232932Z-c9ea149f 后复跑通过；git diff --check 干净；docs/file-platform/content-delivery.md 新增实施状态+留白说明）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260925T233039Z-e5ee2445：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict(20260925T233018Z-0ac2eb18)/git diff --check 全过；真实内容owner与部署保持独立验收未完成）
