# 本机CLI/MCP远程合同消费

## 当前依据与交付边界

新增Server能力需要在本机CLI/MCP中明确发现和消费；不能在适配器复制Server授权、云provider或分析状态。

当前依据：[internal/bridge/sync.go](../../../internal/bridge/sync.go)、[internal/bridge/local_files.go](../../../internal/bridge/local_files.go)、[docs/local-file-experience.md](../../../docs/local-file-experience.md)。检查源码只证明当前实现形状，本文没有执行新增运行时测试或真实后端验证。

本包owner：`cli/drivebridge`；准入：`split-owner`；能力账本：`required / design`。消费方只能通过公开合同；新库、新worker和新命令仍归现有owner，不新增产品仓库。

## 前置、任务依赖与唯一写入

- [drivebridge-upload-batch-recovery-v2](../drivebridge-upload-batch-recovery-v2/design.md)：依赖合同先稳定；本包消费其公开接口，不写其私有状态。
- [drivebridge-cloud-connectors-v2](../drivebridge-cloud-connectors-v2/design.md)：依赖合同先稳定；本包消费其公开接口，不写其私有状态。
- [drivebridge-analytics-search-v1](../drivebridge-analytics-search-v1/design.md)：依赖合同先稳定；本包消费其公开接口，不写其私有状态。
- [drivebridge-agent-organization-v1](../drivebridge-agent-organization-v1/design.md)：依赖合同先稳定；本包消费其公开接口，不写其私有状态。
- [drivebridge-controlled-automation-v1](../drivebridge-controlled-automation-v1/design.md)：依赖合同先稳定；本包消费其公开接口，不写其私有状态。

本包内部主链为1.1→1.2→2.1→2.2→3.1→3.2→4.1→4.2→5.1；5.2规模验收和6.1文档在5.1后推进，6.2等待二者完成。前置包的1.2合同稳定即可启动设计；运行集成需对应应用服务与入口已通过定向验证，不能以仅有spec满足运行依赖。没有子Agent自动授权，后续实现按拥有路径串行写；独立规格可分别审阅，稳定diff后才验收。

## 用户流程与状态

场景：纯MCP发现→限定文件上传→原ID恢复→授权快照查询。

```mermaid
flowchart LR
Discover --> ScopeCheck
ScopeCheck --> SDK
SDK --> OriginalOperation
OriginalOperation --> Resume
```

只读检查、执行意图和最终结果分开；partial保留逐项结果，结果未知先回读原ID。恢复不更换来源、身份、目标或幂等键；取消不等于删除已完成副作用。

## 数据与版本

只保存现有本机配置、绑定与传输状态；远程batch/plan/query/operation均保留Server ID。DuckDB本机执行复用Server拥有的共享typed analytics adapter与授权快照格式。

新增关系字段采用可空或安全默认值，新增索引可回填；不删旧列、不改旧ID或URI。引用资源通过application service建立，配置/计划/回执/清单不由Agent手写。

## 公开接口与具体行为

| HTTP / 入口（规划，非已上线） | MCP / 客户端面 | SDK / 应用服务操作（规划） | 输入与结果 |
| --- | --- | --- | --- |
| drivebridge remote --action ACTION | 现有远程适配路径 | Server公开SDK | action仅来自实际tools/list和能力表 |
| 规划命令族：upload、batch、query、organize | 规划命令族 | Server公开SDK | 与Server同语义，local/remote模式明确 |
| MCP upload_batch_*/files_query/organize_plan/organize_apply | 规划工具 | Server公共合同 | 不shell out产品CLI，不读取Server私有库 |
| drivebridge analytics snapshot query | 规划可选终端路径 | 共享typed快照适配 | 不提供任意SQL、路径或默认安装DuckDB |

表中GET/POST和花括号动作表示分别注册的路由，顿号分隔的是独立工具/SDK名称，不能作为字面路径或工具别名调用。表中的SDK名称是待实施应用操作名，不表示已有导出符号。落地时CLI/MCP/HTTP必须调用同一应用服务，SDK无付费重试、权限扩张或自动切换后端逻辑；开发者不能把接口表当作当前可执行命令。

- 本机文件读取仅允许配置roots内普通文件；远端MCP不接受本机绝对路径。网页上传或本机adapter传HTTP字节，MCP只传引用。
- server不可用时不自动改走另一账号、本机rclone或其他云目标；同一意图的恢复只读原instance和operation。
- tools/list、inputSchema与capabilities联合发现。旧server缺新能力返回unsupported，不注册假可用工具；本机标签不冒充远程认证Agent。
- 凭据只在用户级受保护配置/现有secret引用中读取；不写回spec、回执、快照或stdout。CLI命令与错误英文，项目文档中文。
- local sync/mount权限保持原边界；远程organize_apply只有Server授权有效才可调用，不因本机--enabled而获得服务端权限。
- 接收快照需显式export权限；DuckDB进程无网络和任意文件权限；缺依赖只影响此可选分析路径。

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
  "action": "operation_lookup",
  "arguments": {
    "action": "upload_batch_create",
    "idempotency_key": "batch-example",
    "original_request": {"space_id": "SPACE_ID", "conflict_policy": "fail"}
  }
}
```

结果：

```json
{
  "instance_id": "SERVER_INSTANCE",
  "batch_id": "BATCH_ID",
  "operation_id": "SERVER_OP_ID",
  "recovery_owner": "server"
}
```

这是远程动作适配示例，不是新Server授权协议；远程ID必须原样保留。本机file_ref与Server固定URI按类型区分，跨模式只经显式导入转换。缺工具时不给虚假的可执行next action。

| 新码/沿用码 | HTTP状态（旧码保留现有映射） | 恢复行为 |
| --- | --- | --- |
| `unsupported` | 400 | 先重新发现服务能力，不猜动作或自动安装。 |
| `path_not_allowed` | 400 | 来源必须在本机允许root，不能扩大路径。 |
| `analytics_backend_unavailable` | 503 | 可选DuckDB缺失不影响上传。 |

本包规模验收固定为：消费Server万项批次分页fixture与100GiB流式测试入口，Adapter内存有界；无CLI MCP场景、旧Server、原ID恢复及快照范围隔离；真实云不重复计作本机证据。

## 测试规格与故障注入

| 场景 | Requirement | 正常与反例 | 必须断言 |
| --- | --- | --- | --- |
| 1 | 共享合同与原ID | 远程上传响应丢失；Server新工具缺失 | 恢复原Server Operation。 / unsupported并给出实际缺口，不猜别名。 |
| 2 | 本机文件与纯MCP | 远程Agent没有本机adapter；本机来源越过root | 说明网页/HTTP入口，不假装可读。 / 拒绝且不传输字节。 |
| 3 | 可选快照分析 | 已授权快照且DuckDB可用；没有export授权 | 返回同代次统计并声明离线副本。 / 拒绝，不退化为全库导出。 |

还需覆盖相同请求重放、并发编辑、权限撤销、跨租户、来源变更、进程重启、响应丢失，以及旧客户端缺新字段时的行为。性能验收只覆盖本包相关部分，统一目标见 [本owner验收与路线文档](../../../docs/file-platform-evolution.md)。

实施任务先加入明确测试入口后运行以下命令；无匹配测试或输出no tests to run属于失败，不能以空运行关闭任务。当前文档轮次不执行产品测试。

```bash
python3 scripts/check.py -- go test ./internal/bridge -run TestFPRemoteSurfaces -count=1
python3 scripts/check.py -- go test -race ./internal/bridge -run TestFPRemoteSurfaces -count=1
```

集成证据由现有runner写入本owner `temp/integration-test-runs/<run-id>/`，包含summary.json、command.txt、stdout.log、stderr.log、env.json与artifacts/；失败同样保留并脱敏。真实存储、身份、模型费用与部署分别验证，不由fixture代替。

代码、定向测试和文档稳定后，才执行本包所需最终门：

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge
openspec validate drivebridge-remote-surfaces-v2 --strict --no-interactive
git diff --check
```

## 迁移、回滚与完成定义

- 当前只新增设计文档，运行时迁移未执行；实现按expand-then-contract，兼容既有客户端、元数据和凭据来源。
- 默认关闭新能力；关闭开关停止新意图，保留原Operation、文件、版本、计划、回执与审计。新版本已产生业务写入时，不允许回滚为丢弃这些写入的旧数据库快照。
- 本包负责自身配置与状态，不替消费者执行部署、发消息、真实云写入、付费调用或发布；这些门由相应owner独立验收。
- 设计完成=proposal/design/spec/tasks可追溯且strict通过。实现完成=任务指定运行证据通过。真实后端验证与发布部署是后续独立事实。

## 范围记录

2026-09-25：按用户确认的百万文件/万文件批次/100GiB目标和分层数据方案建立本包；目标未验收，required能力仅分期，不删除。
