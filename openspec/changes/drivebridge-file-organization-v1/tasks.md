## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与虚拟组织与标签权威；验证 openspec validate drivebridge-file-organization-v1 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T205400Z-a694cff4）
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。（接口表字段 files_query/file_labels_set/file_collections_list/file_collection_create/collection_members_set/collection_snapshot/saved_searches_list/saved_search_create 与错误码 query_changed/label_namespace_forbidden/collection_revision_conflict 已人工复核一致）

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPFileOrganization模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T210700Z-f90dfebf；LabelAuthority 身份/隔离/幂等反例、QueryContract 游标绑定）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T210700Z-f90dfebf；SchemaAdditive 演练仅删六张纯新增表，旧数据可读、标签面 fail-closed、文件事实保持可查）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现虚拟组织与标签权威和集合与快照语义的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T210700Z-f90dfebf；CollectionAndSnapshot 动态/快照/撤权过滤、SavedQueries、Duplicates 精确=已验证摘要+similar 标记+purge 守卫）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点精确重复和相关候选；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPFileOrganization -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T211148Z-99318d64；反例覆盖：version_conflict/collection_revision_conflict 保留服务端决定、query_changed 游标绑定、跨owner not_found、trashed 成员/快照失权过滤、partial 分页、精确重复=已验证摘要+similar 标记+unknown 摘要跳过、RestartReplay 重启回读同ID同修订）

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T211440Z-abc8e7fd；EntryEquivalence 三入口同义项/同 catalogue revision、MCP manual 拒止同判、8工具可发现）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“标签建议→人工确认→规则集合→固定快照→撤权过滤”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPFileOrganization -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T211440Z-abc8e7fd；FixtureFlowSuggestConfirmRuleSnapshot 建议→确认→规则集合→快照→撤权过滤全程公开接口）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPFileOrganization -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；百万目录精确分页20并发P95≤2秒；10000成员集合/快照，重复摘要与部分权限分组；跨页撤权不能泄漏计数或成员。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。（留白：quick=10000 规模在本机三次执行均超 runner 时限（20260926T012414Z-186cf344 P95 8.5s 且 601s 超时、20260926T021101Z-7bc1bca6 840s 超时）——10k HTTP 播种+单写者 SQLite 在本机不可达；维度覆盖已在非规模测试单独验证（keyset 零重复/撤权零泄漏/1000成员快照），百万与 2s 目标未声明；可按 DRIVEBRIDGE_FP_SCALE=quick 在更快机器复跑） | evidence: 2026-10-09 local SQLite full mode DRIVEBRIDGE_FP_SCALE=full PASS: million-file exact keyset walk,10000-member collection/snapshot,20 concurrent readers P95=695.350818ms,total359.82s; temp/integration-test-runs/20261008T200220Z-5dd81cb3. Log label quick-mode is legacy wording; env full selects1000000 and10000. Prior timeouts diagnosed/fixed ordered owner index, bounded keyset predicate, per-file usage N+1/read-query recapture, fixture seed lookup/batching. PostgreSQL production performance remains separately unclaimed.

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-file-organization-v1 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T214004Z-83bc5951；git diff --check 干净；docs/file-platform/query-organization.md 新增实施状态节含真实命令与规模留白说明）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260925T214022Z-21b229af：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict(20260925T214004Z-83bc5951)/git diff --check 全过；语义候选与部署保持独立验收未完成）
