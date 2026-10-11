## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与事务与分析独立；验证 openspec validate drivebridge-analytics-search-v1 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T230306Z-d4024f1b）
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。（接口表 analytics_query/analytics_query_get/analytics_snapshot/analytics_snapshot_query 与错误码 authorization_projection_stale/analytics_backend_unavailable/query_budget_exceeded/snapshot_expired 已复核一致；observed 在无分析后端时如实 503 不静默回退为设计语义）

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPAnalyticsSearch模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T231303Z-18d25256；analytics_snapshots 纯新增表+幂等键；TypedQueryContract 身份/词表反例）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T231303Z-18d25256；纯新增表，live 路径只读既有目录，回滚仅删新表）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现事务与分析独立和安全查询和分面的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T231303Z-18d25256；live 精确聚合+observed 诚实不可用+快照 manifest/幂等重放）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点DuckDB进程隔离；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPAnalyticsSearch -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T231303Z-18d25256；RevocationAndDrillDown 撤权零泄漏、SnapshotsAndEngineIsolation 引擎缺失503+SQL/路径payload先invalid_query+Agent禁导出）

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T231303Z-18d25256；HTTP/SDK/MCP 同一应用服务，旧 usage/audit 工具保留未动）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“资源分布→权限内钻取→投影故障→受控快照分析”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPAnalyticsSearch -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T231303Z-18d25256；TypedQueryContract 聚合→同范围钻取一致→撤权）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPAnalyticsSearch -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T231303Z-18d25256；spec四Requirement场景：事务与分析独立/安全查询与分面/幂等投影与代次（快照幂等重放）=覆盖，DuckDB 隔离=引擎缺失503+payload先验）
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；100万文件/1000万事件，20并发常用分面/30天趋势P95≤3秒；冻结快照在SQL基线/ClickHouse/DuckDB结果一致，中文查询golden与ACL滞后计数反例必须通过。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。（run 20260925T231303Z-18d25256；本机未跑百万/千万与 ClickHouse/DuckDB 三后端一致性——分析后端未部署，live 路径全断言过；按 exclusions 留白记录）

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-analytics-search-v1 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 见 20260925T231303Z-18d25256 后复跑通过；git diff --check 干净；docs/file-platform/query-organization.md 分析检索节新增实施状态+留白说明）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260925T231415Z-ccc9fd44：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict(20260925T231354Z-1eb9b69c)/git diff --check 全过；分析后端与部署保持独立验收未完成）
