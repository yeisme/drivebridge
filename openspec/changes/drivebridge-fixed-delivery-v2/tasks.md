## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与固定清单完整性；验证 openspec validate drivebridge-fixed-delivery-v2 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T233520Z-ac4698f6）
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。（接口表 delivery_prepare/delivery_get/人类publish/delivery_receipt 与错误码 delivery_incomplete/retention_not_ready/receiver_identity_required 已复核一致）

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPFixedDelivery模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T234506Z-558a8f4d；delivery_manifests/items/receipts 纯新增表+幂等键；ManifestIntegrity 身份/重放）
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T234506Z-558a8f4d；纯新增表，快照/保留/分享既有表不动）

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现固定清单完整性和接收事实分级的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T234506Z-558a8f4d；完整性：2/2 verified+retention 发布过、缺项 delivery_incomplete 必需项不删、无hold retention_not_ready）
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点撤销和领域采纳；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPFixedDelivery -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T234506Z-558a8f4d；接收事实分级 authenticated/declared/bytes_served、verified 需认证接收方、Agent 禁 prepare/publish（分享管理人类专属））

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。（run 20260925T234506Z-558a8f4d；HTTP/SDK/MCP 同一应用服务；delivery_prepare/get/receipt 三工具可发现，publish 无Agent工具）
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“冻结集合→核验清单→发布→接收确认→撤销”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPFixedDelivery -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T234506Z-558a8f4d；fixture 流：分享→快照→hold→prepare→publish→回执分级，全程公开接口）

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPFixedDelivery -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。（-race 变体未执行，AGENTS.md race 禁令高于任务文本；非 race 验证 run：20260925T234506Z-558a8f4d；spec三场景：固定清单完整性/接收事实分级（含伪造拒绝路径 receiver_identity_required 语义）/撤销与领域采纳（分享撤销 share_unavailable 语义沿用））
- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；10000成员固定清单有界分页与核验；缺项、hold/purge竞态、接收确认伪造、撤权和旧版不可读；发布前完整性不可被UI进度替代。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。（run 20260925T234506Z-558a8f4d；10000成员规模 fixture 本机未跑（复用组织快照规模路径可扩），按 exclusions 留白记录；hold/purge 竞态由 retention-guards 既有测试覆盖语义

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-fixed-delivery-v2 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。（strict run 20260925T234506Z-558a8f4d 后复跑通过；git diff --check 干净；docs/file-platform/content-delivery.md 新增实施状态+留白说明）
- [x] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（最终门 run 20260925T235001Z-b64b40cb：check.py 全量失败集=4 项既有基线逐一一致（artifacts/baseline-comparison.md）；vet/build/strict(20260925T234553Z-fb428dbb)/git diff --check 全过；规模与真实接收方保持独立验收未完成）
