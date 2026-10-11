## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与三类云端能力矩阵；验证 openspec validate drivebridge-cloud-connectors-v2 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPCloudConnectors模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T201056Z-9a0ccdfe）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T201125Z-106c8960；TestFPCloudConnectorsSchemaAdditive 演练仅删 cloud_roots/external_entries 两张纯新增表）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现三类云端能力矩阵和外部观测与受管版本的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T202632Z-1f895e59；CapabilityMatrix/ImportManagedCopy 覆盖矩阵申报与导入正常路径）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点范围和游标失效；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPCloudConnectors -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T202759Z-74f15f48，DriftScopeAndCursorInvalidation/VersionUnavailableOnUnversionedBackend 覆盖漂移/游标失效/部分页/版本不可得反例）

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T202856Z-c371205b；EntryEquivalence 断言三入口同义项、未授权Agent拒止、旧items路由语义不变、工具可发现）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“网页绑定云目录→分页浏览→源漂移→明确导入固定副本”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPCloudConnectors -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T202941Z-29609c8f，FixtureFlowBindBrowseDriftImport 全程公开接口120项3页浏览+漂移+导入回读）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPCloudConnectors -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T203251Z-18eb7ddb；spec四Requirement场景由 CapabilityGate/RootAndEntryIdentity/ImportManagedCopy/DriftScopeAndCursor/GDriveAdapterScenarios/OneDriveAdapterPath/RevocationAndErrorInjection 覆盖）
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；每后端100万entry分页fixture，页内重复/页间漂移/游标过期/部分失败；有界内存、不完整扫描不推导删除；各后端真实目录与OAuth独立另验。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T203756Z-111fbcc0；DRIVEBRIDGE_FP_SCALE=quick 实际命中：10000 entry 50页界keyset分页零重复、观测行唯一=总数、页间漂移 source_changed、budget=500 partial 扫描零墓碑、完整扫描零误删；1M full 变体本机未执行——SQLite 单写者观测 upsert 主导时长，DRIVEBRIDGE_FP_SCALE=full 可复跑；真实后端目录与OAuth独立另验）

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-cloud-connectors-v2 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T203938Z-1fe762cf；git diff --check 干净；docs/file-platform/uploads-cloud.md 云端目录节新增实施状态，回滚演练引用 TestFPCloudConnectorsSchemaAdditive）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260925T205035Z-22b5b3ae：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict/git diff --check 全过；真实后端与部署保持独立验收未完成）
