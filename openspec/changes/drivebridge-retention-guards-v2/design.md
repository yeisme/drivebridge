# 服务端版本保护与保留计划

## 当前依据与交付边界

本机同步不能拥有Server历史版本保护；固定交付、在途传输和内容引用需要服务端独立保护合同。

当前依据：[internal/service/http.go](../../../server/internal/service/http.go)、[internal/service/tools.go](../../../server/internal/service/tools.go)、[internal/service/model.go](../../../server/internal/service/model.go)、[docs/contract-map.md](../../../docs/server/contract-map.md)。检查源码只证明当前实现形状，本文没有执行新增运行时测试或真实后端验证。

本包owner：`cli/drivebridge/server`；准入：`fit`；能力账本：`required / design`。消费方只能通过公开合同；新库、新worker和新命令仍归现有owner，不新增产品仓库。

## 前置、任务依赖与唯一写入

- [drivebridge-team-runtime-projections-v1](../drivebridge-team-runtime-projections-v1/design.md)：依赖合同先稳定；本包消费其公开接口，不写其私有状态。

本包内部主链为1.1→1.2→2.1→2.2→3.1→3.2→4.1→4.2→5.1；5.2规模验收和6.1文档在5.1后推进，6.2等待二者完成。前置包的1.2合同稳定即可启动设计；运行集成需对应应用服务与入口已通过定向验证，不能以仅有spec满足运行依赖。没有子Agent自动授权，后续实现按拥有路径串行写；独立规格可分别审阅，稳定diff后才验收。

## 用户流程与状态

场景：清理预览→交付保护→并发purge拒绝→期满重新计划。

```mermaid
flowchart LR
Preview --> Recheck
Recheck --> Eligible
Recheck --> Protected
Eligible --> Cleaning
Cleaning --> Complete
```

只读检查、执行意图和最终结果分开；partial保留逐项结果，结果未知先回读原ID。恢复不更换来源、身份、目标或幂等键；取消不等于删除已完成副作用。

## 数据与版本

RetentionHold、RetentionPlan、RetentionItem、RetentionPolicy；hold绑定file/version、用途、关联operation/share、expires_at。purge事务与hold创建/解除共享同一版本锁和revision条件。

新增关系字段采用可空或安全默认值，新增索引可回填；不删旧列、不改旧ID或URI。引用资源通过application service建立，配置/计划/回执/清单不由Agent手写。

## 公开接口与具体行为

| HTTP / 入口（规划，非已上线） | MCP / 客户端面 | SDK / 应用服务操作（规划） | 输入与结果 |
| --- | --- | --- | --- |
| POST /api/v1/retention/holds | retention_hold（按用途与权限） | CreateRetentionHold | 固定version、purpose、expiry、source operation；额度校验 |
| GET/POST /api/v1/retention/plans | retention_preview | PreviewRetention | policy revision、范围、类别，默认只预览 |
| POST /api/v1/retention/plans/{id}/execute | 用户/已配置系统策略；无Agent purge | ExecuteRetention | 人类确认或既有明确自动清理策略，复核hold |
| GET /api/v1/retention/holds/{id} | retention_hold_get | GetRetentionHold | 当前保护状态与安全来源 |

表中的SDK名称是待实施应用操作名，不表示已有导出符号。落地时CLI/MCP/HTTP必须调用同一应用服务，SDK无付费重试、权限扩张或自动切换后端逻辑；开发者不能把接口表当作当前可执行命令。

- 保留保护不是永久保存承诺；发布前确认有效期限与逻辑容量，hold不重复计算同一版本占用，但会阻止释放。
- 当前文件、历史版本、回收站、上传暂存分别清理；同一blob仍有其他引用时不删除字节。CAS引用计数为候选，清理前事务重验。
- purge与新hold竞态必须互斥；purge已开始时创建hold失败，不能发布虚假交付保证。已保护版本需先显式撤销相关交付或等待期满。
- 自动清理保留现有已启用策略的范围，不给Agent新增purge工具。新增类别策略默认关闭，策略修改需人类身份。
- tombstone先禁止访问，再异步清理对象和投影；取消清理不会复活已删除版本。不能凭ClickHouse快照直接判定删除资格。

## 共同合同约束

本包为 **设计与待实施合同**。本文规划路径、工具、字段、类型和状态尚未上线；不能把 spec strict 成功当成实现、真实后端或部署验收。既有接口须先从实际 tools/list、inputSchema、CLI help 与公开API确认。新能力默认关闭，部署配置未启用时报告 unsupported/blocked。

- HTTP 沿用各 owner 现有错误信封及认证；身份从会话/令牌推导，body 中的 tenant/actor/project 只能收窄选择范围。CLI 保留 human/--json/--agent 与已实现输出选项，不为旧入口改变必填字段或输出类型。
- 新写请求采用 Idempotency-Key 或该工具对应 idempotency_key，作用域为 instance+tenant+actor+action，指纹覆盖所有业务输入；同键同指纹复用，同键异指纹拒绝。已有入口保持原幂等作用域，禁止重解释旧键。编辑/执行携 expected_revision/expected_version；过期拒绝，不自动重算或批准。
- DriveBridge新分页默认50、最大200；Manju既有查询保留默认20/最大100及8秒预算，旧分页不变；游标不透明，绑定规范查询、身份范围、排序与数据代次。所有页重新授权；跨查询、跨身份游标拒绝。精确列表使用keyset稳定排序，以ID破同值；语义Top-K不是全集，异步导出使用独立原Operation。
- 新元数据与类型由应用服务/CLI生成，GORM为Go业务持久访问层；禁止在handler/SDK适配器拼接SQL。迁移、专用队列领取和分析器例外集中adapter、绑定参数、限定标识符，并在实现review列明理由。
- 保持现有 File/Version/Operation、URI、旧状态枚举与字段含义。本文流程节点描述新增流程或展示阶段，不是把旧 Operation.status 替换成一套新枚举。未知外部结果以新增 certainty/恢复信息表达，不伪报失败或成功。
- 整理、传输、内容处理与领域采纳是不同事实；不会因保存文件批准生成、付费、发布或采纳。回执不含凭据、临时URL、原始模型请求或完整内部推理。
- 基本错误：invalid_query/invalid_request=400，forbidden=403，not_found=404，idempotency_conflict/version_conflict/plan_stale=409，cursor_expired=410，unsupported=400（沿用既有Server映射），budget_exceeded=429，dependency_unavailable=503。保留已有错误码及状态码，专有新码见本包表；不得把旧同名码改义。


## 输入、结果与错误样例（规划）

以下标识符为说明性占位，不是可执行请求；实现测试由fixture/CLI生成合法ID。HTTP成功结果沿用owner现有信封，下面展示payload；CLI/MCP映射保持各自旧信封。示例不授予权限。

输入：

```json
{
  "file_id": "FILE_ID",
  "version_id": "VERSION_ID",
  "purpose": "fixed_delivery",
  "source_ref": "SHARE_ID",
  "expires_at": "2026-10-25T00:00:00Z",
  "idempotency_key": "hold-example"
}
```

结果：

```json
{
  "hold_id": "HOLD_ID",
  "revision": 1,
  "state": "active",
  "version_id": "VERSION_ID",
  "expires_at": "2026-10-25T00:00:00Z"
}
```

RetentionPlan固定policy revision和候选version集合；计划只预览，执行逐项重新验证hold、引用和当前权限。hold的来源必须是本主体可管理资源，不接受任意外部字符串充当保护授权。

| 新码/沿用码 | HTTP状态（旧码保留现有映射） | 恢复行为 |
| --- | --- | --- |
| `version_protected` | 409 | 先处理合法保护关系或等待到期，不能强制绕过。 |
| `purge_in_progress` | 409 | 拒绝新hold和保证发布。 |
| `retention_plan_stale` | 409 | 重新预览当前版本和引用。 |

本包规模验收固定为：百万版本/10000holds，20并发hold与purge冲突；共享blob多引用、到期重算和取消清理；分析滞后不影响保护正确性。

## 测试规格与故障注入

| 场景 | Requirement | 正常与反例 | 必须断言 |
| --- | --- | --- | --- |
| 1 | 保护与清理互斥 | 活跃交付hold；purge与hold并发 | 跳过并说明保护来源。 / 只有一方获得资格，不产生悬空保证。 |
| 2 | 容量和引用安全 | 两个版本引用同blob；分析投影显示零引用但事务仍有引用 | 另一版本可读，物理字节保留。 / 以事务真源拒绝删除。 |

还需覆盖相同请求重放、并发编辑、权限撤销、跨租户、来源变更、进程重启、响应丢失，以及旧客户端缺新字段时的行为。性能验收只覆盖本包相关部分，统一目标见 [本owner验收与路线文档](../../../docs/server/file-platform/verification.md)。

实施任务先加入明确测试入口后运行以下命令；无匹配测试或输出no tests to run属于失败，不能以空运行关闭任务。当前文档轮次不执行产品测试。

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPRetentionGuards -count=1
python3 scripts/check.py -- go test -race ./internal/service -run TestFPRetentionGuards -count=1
```

集成证据由现有runner写入本owner `temp/integration-test-runs/<run-id>/`，包含summary.json、command.txt、stdout.log、stderr.log、env.json与artifacts/；失败同样保留并脱敏。真实存储、身份、模型费用与部署分别验证，不由fixture代替。

代码、定向测试和文档稳定后，才执行本包所需最终门：

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
openspec validate drivebridge-retention-guards-v2 --strict --no-interactive
git diff --check
```

## 迁移、回滚与完成定义

- 当前只新增设计文档，运行时迁移未执行；实现按expand-then-contract，兼容既有客户端、元数据和凭据来源。
- 默认关闭新能力；关闭开关停止新意图，保留原Operation、文件、版本、计划、回执与审计。新版本已产生业务写入时，不允许回滚为丢弃这些写入的旧数据库快照。
- 本包负责自身配置与状态，不替消费者执行部署、发消息、真实云写入、付费调用或发布；这些门由相应owner独立验收。
- 设计完成=proposal/design/spec/tasks可追溯且strict通过。实现完成=任务指定运行证据通过。真实后端验证与发布部署是后续独立事实。

## 范围记录

2026-09-25：按用户确认的百万文件/万文件批次/100GiB目标和分层数据方案建立本包；目标未验收，required能力仅分期，不删除。
