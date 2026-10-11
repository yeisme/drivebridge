## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与复制与来源清理分离；验证 openspec validate drivebridge-cross-storage-transfer-v1 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T214551Z-449c0dae）
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。（接口表 transfer_plan/transfer_plan_get/transfer_execute/cleanup-source 无Agent工具、错误码 plan_stale/target_conflict/source_cleanup_unsupported 已复核一致；overwrite 需 per-target expected_version+人工确认故不在计划词表，占位目标冲突走 target_conflict）

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPTransferPlans模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T222615Z-bf43189b；IdentityAndGate 身份/幂等指纹/跨owner/默认关 unsupported）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T222615Z-bf43189b；SchemaAdditive 仅删 transfer_plans/transfer_items 两张纯新增表，旧数据可读、面 fail-closed）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现复制与来源清理分离和逐项幂等与结果未知的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T222615Z-bf43189b；CopyExecuteAndVerify 逐项复制+摘要验证+managed ref+重放不重复；修复两处 pool-in-tx 死锁：createUpload/publishUpload rename 路径改 allowOn(tx)）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点跨空间权限与补偿；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPTransferPlans -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T222615Z-bf43189b；DriftPauseAndCleanupSeparation/AuthorizationsAndConflicts/RestartReplay 覆盖 source_changed 暂停单项、cleanup 分离+copied_source_pending+人类专属、跨空间独立确认、target_conflict、重启回读）

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T222615Z-bf43189b；EntryEquivalence 三入口同义、cleanup 无 Agent 工具、三工具可发现）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“多云复制→部分失败→目标读回→显式源清理”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPTransferPlans -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T222615Z-bf43189b；FixtureFlowCopyPartialReadbackCleanup 多源复制→部分失败→原ID目标读回→人类显式源清理）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPTransferPlans -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；10000项部分成功/恢复/取消，独立100GiB流式云间复制；核对目标唯一性、源删除需单独授权及有效流量/验证流量口径。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-cross-storage-transfer-v1 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T222710Z-c11b980b；git diff --check 干净；docs/file-platform/uploads-cloud.md 跨存储节新增实施状态+真实命令+规模留白）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260925T225946Z-d567c6e9：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict(20260925T222710Z-c11b980b)/git diff --check 全过；规模fixture按 readiness 先例 env-gate（标准套件外），100GiB 真实流与部署保持独立验收未完成）
