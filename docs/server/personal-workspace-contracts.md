# 个人多 Agent 工作台接口与行为合同

状态：2026-10-01 设计修订稿，新增路径尚未实现。唯一 owner 为 `cli/drivebridge`，HTTP/权限/持久化位于 `server/`；前端 [PRD](../web/product/personal-agent-workspace-prd.md) 与 [OpenSpec](../../openspec/changes/drivebridge-personal-workspace-contracts-v1/README.md) 共同引用本文。不把原型内存字段直接当 wire schema。

## 1. 可复用源码与增量边界

既有基线核对 `server/internal/service/http.go`、`*_http.go`、`agentorg*.go`、`organization_model.go`、`model.go`、`auth.go` 及 `server/sdk/`。以下“已有”指工作树存在合同/实现，尚未在本轮重跑运行验收。

| 能力 | 已有路径/代码 | 缺口与本包责任 |
| --- | --- | --- |
| 登录/当前用户 | `/auth/mode`、`/auth/key`、OIDC、`GET /api/v1/me` | 原样保留；新 UI 同源 cookie+CSRF |
| 文件与固定版本 | files、versions、content、references；sdk/client.go | 首页/知识/资源只投影原 File/Version；文档乐观保存补充 |
| 上传与恢复 | uploads、operations、upload-batches、operations/lookup | 受限 Agent 回存加 staging 限制，原任务不重造 |
| 目录/查询/组织 | cloud entries、file-queries、labels、file-collections、snapshots、saved-searches | 收藏、属性定义、资源描述与引用投影补充 |
| 整理与传输 | organization-plans、organization-policy、transfer-plans | 新任务页投影；原确认/执行/补偿服务继续拥有写入 |
| 交付/讨论 | delivery-manifests、delivery-receipts、threads、private-note、archive | 收件“接收/拒绝”和归档去向补充；领域采纳不由本服务决定 |
| Agent | GET/POST agents、revoke；Agent 仅五类 scope、默认 7 天 | 新增细粒度 grants；保留旧响应数组、scope 与默认值 |
| 审计/自动化/内容 | audit、changes、automation-rules、content-bindings/queries | 调用分页投影；内容不自动处理，运行时不可用诚实返回 |

不新增通用任务引擎、Agent 编排器、Provider SDK、MCP 网关或平行文件数据库。本机 MCP/CLI 从 Server 的 application/SDK 接入；不 shell out，不读取私有 DB。CLI/MCP、HTTP 与 Web 已合入同一 owner，按模块协作而非跨 owner 交接；外部领域仍只消费公开合同。

## 2. 协议公共规则

- 新路径前缀 `/api/v1`，响应保持已有 `data` envelope；失败保持 `error.code/message`，message 为安全英文。旧端点参数、返回结构和状态码不改。
- 新列表统一 `items, next_cursor, observed_at`；`total` 可空，仅精确计数时提供，附 `total_kind: exact|unknown`；默认 limit=50，范围 1–200。cursor 绑定实例、当前主体、租户、空间、授权修订、筛选、排序与读取代次；失效 410，不静默重复首批。
- ID/ref 都是 opaque string；时间 RFC3339 UTC，显示层用用户时区。revision 是非负安全整数，更新以 `expected_revision` CAS；正文保存使用 `base_version_id`。不改变旧 uint64 字段 wire 类型。
- Cookie 写请求继续校验 `X-CSRF-Token` 与 origin。Agent bearer 身份来自服务端 authenticate；body 不接受 owner/actor/admin 来选择身份。
- 新创建/决策/执行请求要求 `Idempotency-Key`。作用域 instance+tenant+authenticated actor+action+key，绑定规范化请求摘要；相同请求回原资源，不同内容 409 `idempotency_conflict`。键在发送前持久化为非秘密恢复身份。
- 列表、详情、计数、引用、事件和下载均按同一权限过滤。新个人工作台资源对跨用户/跨租户/不可见 ID 使用 404，合法可见对象缺动作权限 403；旧接口不借本包变更其错误合同。
- 所有 response 不含 token hash、实例密钥、DSN、raw Provider payload、内容提示词或签名 URL；只有明确的凭据一次交付响应允许 secret，并设置 no-store、禁止日志/证据捕获。
- capability 扩展 `actions`，保留旧键。未实现动作 422 `unsupported_action`，未配置依赖 503 `dependency_unavailable`，无权 403；不得把源码存在当可用。
- Go DTO 是唯一 wire 合同真源，本文定义待落 DTO 的行为。B01.2 交付一个统一生成入口：从 `server/` 的 DTO、路由/权限/错误登记生成机器 schema、`web/` TypeScript 类型和契约 fixture；HTTP/SDK/MCP 测试消费相同 fixture，不能另手写 TS 字段。显式 `expiry_mode` 与 nullable `expires_at` 必须生成 required+nullable，不能变为省略字段。B01.3 交付漂移校验：确定性重生成到临时目录、比较受管产物、检测旧响应兼容和 nullable/枚举差异；改 DTO 未重生成、改生成物、fixture 不匹配均失败。新工具及真实生成/检查命令在 B01 实现后登记，本轮不虚构已经存在的命令。本文中的新 HTTP 路径是设计，不能直接探测生产。选型固定为 Go DTO/路由登记 → invopop/jsonschema 的 JSON Schema 2020-12 + OpenAPI 3.1 → openapi-typescript；类型化 Go 样例产生 fixture，Ajv 8/ajv-formats 仅开发期校验。拟议 make contracts/contracts-check 尚未实现；路径、依赖及漂移反例见 [D13 合同生成](../architecture/personal-workspace-technical-decisions.md)。PATCH 未提交/null/值三态必须保留，不能因普通指针解码丢失 presence。

## 3. 工作台聚合与任务投影（新增）

| HTTP | 请求 | data | 权限/写入 |
| --- | --- | --- | --- |
| GET /workspace/home | space_id 必填；recent_limit 默认 6 最大 20 | space_id、recent_files:FileRef[]、collections、pending_counts、usage、sections、observed_at | 人类可读空间；每段独立 ready/partial/unavailable；不触发扫描或模型 |
| GET /workspace/tasks | space_id、kind、state、cursor、limit；可选 agent_id | TaskProjection 分页 | 人类当前范围；Agent 仅自己的可见操作 |
| GET /workspace/tasks/{kind}/{id} | 固定 source owner ID | TaskProjection + 固定 inputs/outputs + 逐项结果页 | 与原 owner 详情同等授权；无独立执行 endpoint |

`FileRef` 复用 file_id/version_id/ref/space_id/path/mime_type/size_bytes 公开字段；不将 current version 替换成被固定的 version。来源路径可能变化，以 ID/ref 为身份。

B29已实现个人owner的GET `/workspace/home`（不是团队/跨空间聚合）：space_id必填，recent_limit默认6最大20，重复/未知query拒绝。各段独立只读事务并复查human/tenant/scope/root权限，最后再核准入；不是跨段原子快照。recent最多扫描1000候选，按updated/id排序，排除trash/purge/staging，逐文件授权并绑定当前观察时固定Version；缺版本partial，超预算unavailable。集合只公开最近20个ID/name/kind/revision，超限partial，无成员计数/规则执行。pending_counts仅submitted/accepted收件状态（不含任务/建议/审批），恒partial/submission_states_only；1000上限、原操作绑定和路径权限，依赖故障或超预算为null而不是0。usage为Space逻辑used/reserved/quota，不是存储供应商物理容量，异常值unknown。sections各含state/reason/observed_at，错误段清值但不吞健康段；不做正文、扫描任务、模型、存储探测或业务写入。前端/SDK以及完整B03.1待办覆盖仍未完成。

`TaskProjection` 字段：`owner_kind`（operation/upload_batch/organization_plan/transfer_plan/automation_intent）、`owner_id`、`space_id`、`title`、`actor_kind`、可空 `agent_id`、`owner_status`、`ui_state`、`revision`、`progress`、`allowed_actions`、`blocked_reason`、`observed_at`。客户端 key 使用 kind+id，不分配新的领域 task ID。外部 task_ref/caller 仅来源声明，不能转换成可信 actor。

当前 B22 运行时首片只开放人类自身范围 `GET /workspace/tasks?kind=operation`，kind 必填。B25另接受kind=upload_batch，以原batch ID返回同一TaskProjection；B32另接受organization_plan（原plan ID/phase），actor未知不拿确认人当创建者，agent筛选不匹配未知归属；文件数与动作数不同，进度全null，原completed暂unknown/organization_result_unproven，不声称逐动作成功。1000计划/全请求1000项预算，原冻结与当前源路径及跨space权限都核，源不可见整plan隐藏。B33另接受transfer_plan且遵守原TransferPlans gate；target和全部source/已分配target范围及固定version校验，copy/migrate分开，source_cleanup_pending明确待清理，copied/completed结果未独立证明仍unknown；执行anchor不等于creator，进度null。B34新传输条目归档键按plan+item隔离，旧完整origin匹配才恢复原operation，避免跨plan同源目标冲突及历史重复复制。B35另接受automation_intent，核原rule/event/file及当前scope，1000 owner候选预算；历史ActorID不能区分Agent代用户调用，归属未知，completed效果未证保持unknown，读取不运行规则或推进checkpoint。Agent凭据任务列表读取仍未实现。B24另开放人类 `GET /workspace/tasks/operation/{id}`，无query，返回 `task/outputs/output_state`；upload输出只含原file_id/version_id/space_id且额外检查当前文件可读/未回收/未purge，缺失或失权统一unavailable空outputs，不换最新head；未完成not_published，其他operation种类not_projected。不包含inputs、正文、对象键、签名URL或执行动作，读取后使用文件接口仍须重新鉴权。复用持久Operation，不新增任务表；`revision` 为公开投影与原updated时间的SHA-256变更标识，不是递增数字或写前置条件。B23仅对单文件upload提供total=1；未完成且可恢复时pending=1，succeeded/failed未知；completed且同owner/space固定File与Version的ID/size/digest/connection/object匹配时succeeded=1、failed=0、pending=0，最新head不替代原version。历史结果缺失/不匹配或purge中保null而不改原completed，原版本数据库故障返回500；计数是元数据发布事实，不保证当前存储可读。其他kind计数及全部字节值仍null（原Transferred不证明bytes_sent/durable/verified），allowed_actions为空且blocked_reason=owner_actions_not_projected，不发虚构执行链接。原completed才映射completed，failed/interrupted/paused/awaiting_upload映射recoverable，未知owner状态保unknown；收件操作完成不代表人工验收/归档完成。筛选前最多扫描1000候选，超限429 task_scan_budget_exceeded；收件绑定不一致503 task_source_unavailable；数据库失败500而非空列表。默认分页50/max200，授权后筛选/ID排序、游标绑定当前可见投影/身份/范围/查询，源变化410。

B25批量投影先验证当前space根及全部已枚举项读权限，任何项不可见则整批隐藏；候选批量最多1000、整个请求累计项最多1000，超预算429，不伪截断。actor来自绑定的原upload_batch锚点；枚举中total=null，封存总量与行数一致才公开。成功项需confirmed+匹配原单文件operation及固定File/Version，未知成功计数null；failed/pending依据逐项持久状态计数，byte仍null。批量phase=completed且锚点completed、无失败/取消/待处理/未知且成功等于total才ui_state=completed，否则recoverable或unknown。旧operation视图中的completed批量锚点改显示unknown/batch_projection_required，防止与全成功混淆；原owner状态不改变。B26新增人类GET `/workspace/tasks/upload_batch/{id}/items`，cursor/limit默认50最大200，总候选1000。响应batch_id/revision/items/next_cursor/observed_at；条目仅原item_id/relative_path/is_dir/owner_status/result_certainty/可空operation_id，不输出item_key/source_fingerprint/hash/error。当前scope与全部路径授权后再分页；关联operation须核batch/item/owner/space/size/hash及当前路径授权，绑定错配503。confirmed只在原完成+确认+固定结果证明存在时成立，否则unknown；目录不虚构operation。游标绑定主体/tenant/scope/dataOwner/batch/limit及全部公开条目、batch revision/phase，变动410，不使用旧原始item ID游标。条目ID不是恢复operation ID，正文/执行/批量控制仍未接。前端消费和动作另验收。

`ui_state` 为 waiting_confirmation/running/recoverable/completed/failed/cancelled/unknown；保留 owner_status。progress 包含 total/succeeded/failed/pending、可空 bytes_sent/durable_bytes/verified_bytes；completed 必须有原 owner 成功事实。读超时保持 unknown 并回查，不能映射为 failed 后新建操作。

allowed_actions 是稳定 action 标识及目标 owner_ref 的投影，不返回任意可执行 URL。execute/resume/cancel/confirm 仍进入已有 owner 应用服务，点击前复检。分页只读，计数和列表使用同一授权/快照口径。前端采用有界轮询：活动任务约2秒、无活动摘要约30秒，完成请求后才安排下次；后台暂停，回前台重新鉴权。失败按2/4/8/16/30秒带抖动退避并尊重更长的 Retry-After；401/403停止自动轮询。只安全读取自动重试；写入未知先原键/ID查询，授权和归档不提前显示成功。复用安全 changes 通知失效，不新建 SSE/WebSocket/消息队列。

## 4. 细粒度 Agent 授权（新增、旧 Agent 保持兼容）

### 4.1 请求与资源

| HTTP | 请求 | data / 结果 |
| --- | --- | --- |
| POST /agent-grants | name(1–120)、protocol:mcp/http、space_id、connection_id、prefix、actions[]、expiry_mode（省略默认 never）、expires_in_seconds（仅 duration） | 201：GrantView、credential.secret 一次显示、credential.state:issued |
| POST /agent-grants/lookup | original idempotency_key | 当前主体原 GrantView + credential.state；无 secret；无记录 404 |
| GET /agent-grants | space_id、state、q、cursor、limit；可选group_id/ungrouped/pinned/order=pinned | GrantView 分页；组织筛选/置顶排序在分页前执行，组织视图另需既有write可见性并附带同事务organization投影（独立revision）；无组织参数保持原read范围/ID排序且省略organization，缺失不表示未分组 |
| GET /agent-grants/{id} | 无 | GrantView + 最新受权观察摘要，无 secret |
| PATCH /agent-grants/{id} | expected_revision；name、prefix、actions、enabled 可选；期限修改使用 expiry_mode 与条件必填 expires_in_seconds | 更新 GrantView；不接受客户端绝对 expires_at；目标 connection/space 不可原地替换 |
| POST /agent-grants/{id}/revoke | expected_revision | 撤销后 GrantView；同请求幂等 |
| POST /agent-grants/{id}/rotate | expected_revision | 同 grant、新凭据一次显示，旧凭据同事务失效；丢响应同样不能再显示 |
| POST /agent-grants/{id}/rotate/lookup | original idempotency_key | 当前主体原轮转的 grant_id、credential_generation、credential.state=not_replayable；不含 secret，不再次轮转 |
| POST /agent-grants/{id}/changes/lookup | action:disable/enable/revoke/expiry、original idempotency_key | 原事务回执 grant_id/action/result_revision/committed_at；非最新状态，无凭据；无记录404，DB故障500 |
| POST /agent-grants/{id}/checks | expected_revision | 仅校验授权/可发现配置；checked_at、checks[]、connection_state；不执行文件写入 |
| GET /agent-invocations | space_id、agent_id、result、started_after 或 started_at_or_after、started_before、order、cursor、limit | 安全调用记录分页；不返回请求正文、凭据或内部推理 |
| POST /files/{id}/label-suggestions | version_id、labels[]（1–100 个，每项 1–64 字符）；Idempotency-Key | suggestion_id、revision、固定版本、可信 agent_id、labels、state=pending；不直接采纳 |
| GET /files/{id}/label-suggestions | version_id、cursor、limit | 按来源分组的建议；人类看可见文件的建议，Agent 仅自身提交 |
| POST /label-suggestions/{id}/decision | expected_revision、base_version_id、decision:accept/reject、接受的 labels 子集 | 人类专属；原 suggestion 与人工标签的结果，不能覆盖已有人工标签 |

新 grant 仅本人管理，普通 Agent 不能创建、扩权、检查他人凭据或改策略。创建后的 name 以原始显示名处理，不复用 cookie name 的前缀逻辑。新 restriction profile 的期限默认长期：创建省略 `expiry_mode` 等同 `never`；`never` 禁止携带 `expires_in_seconds`，响应 `expires_at: null`。`duration` 必须提供下表枚举秒数；不接受任意数字、零、负值、未知 mode 或绝对过期时间。PATCH 未给期限字段则保持原期限；修改时必须显式 mode（含 duration 时秒数），不能用字段缺失清除期限。无效组合返回 422 `invalid_expiry`，在 B01 注册该新错误，不改变旧端点错误。

| UI 期限 | expiry_mode | expires_in_seconds | 计时依据 |
| --- | --- | --- | --- |
| 长期有效（默认） | never | 不发送 | expires_at 为 null，仍受禁用/撤销/用户权限约束 |
| 1 小时 | duration | 3600 | 服务端成功签发或修改期限的时间 |
| 24 小时 | duration | 86400 | 同上 |
| 7 天 | duration | 604800 | 同上 |
| 30 天 | duration | 2592000 | 同上 |
| 90 天 | duration | 7776000 | 同上 |
| 365 天 | duration | 31536000 | 同上 |

`issued_at` 为首次服务端签发时间，`expiry_changed_at` 为最近一次期限修改成功时间（初次为 null）；duration 的 expires_at 按对应时刻加固定秒数计算，天是 86400 秒而非客户端日历日。幂等重放、连接检查、改名、分组、置顶、禁用/启用和轮转不重置期限。以 server_now >= expires_at 判断过期；同一次变更在事务内固定时间，不受客户端时钟影响。旧 POST `/agents` 的 days 默认 7、允许 1–30 天及旧响应不变，旧记录缺失期限仍按既有规则处理/显式标记未知，绝不推断 never；不以远期日期模拟长期。

label-suggestions 是新 grant 的专用建议通道，S2 由 B16.1/B16.2/F22 交付，不依赖 S3 的收藏/知识资源任务。每组建议绑定提交 Agent、文件版本和原幂等键；同用户 A/B Agent 的建议不能互相覆盖。现有 SetFileLabels 会替换整个 namespace，不能原样用于这个通道。新建议保留独立来源记录，UI 的 suggested 投影可以合并展示，旧 namespace 记录作为 legacy 来源保留。人类采纳以事务内版本/CAS 检查将选定标签加入现有 manual 集合，并记录 accepted_from；不盲目替换整套人工标签。Agent 不能写 manual/rule 或自我采纳。除表内显式 201 外，查询和同步更新成功为 200；确需异步工作时 202 返回原 operation_ref，不返回伪最终结果。

GrantView：`grant_id, agent_id, name, protocol, space_id, connection_id, prefix, actions, restriction_profile, revision, state, expiry_mode, expires_at, issued_at, expiry_changed_at, credential_generation, created_at, updated_at, last_seen_at?, connection_observation?`。`expiry_mode` 必返；`expires_at` 和 `expiry_changed_at` 必返且可 null；credential_generation 为非秘密递增代次。state=active/disabled/expired/revoked；last_seen_at 来源已认证调用，不等于 Agent 正在线。connection_observation 区分 unknown/observed_ok/observed_error，携带 observed_at；检查配置成功不能升级成远端 Agent 可达。

对只读新 grant，前端默认仅选 files.read；建议和收件分别选择。创建成功 secret 不入 URL/localStorage/IndexedDB/截图/trace，关闭后无法重看。若创建响应未知，先 lookup 原键；轮转响应未知则查 rotate/lookup 原键并 GET grant 确认当前代次。已成功且 secret 丢失时显示“重新签发”，人类确认后以新键和最新 revision 再 rotate，禁止自动连环轮转或悄悄再建一个 Agent。过期/已撤销 grant 不允许 rotate，重新接入须新 grant。

组织列表投影不新增状态：group_id与ungrouped互斥，pinned/ungrouped只接受true/false，order仅支持pinned。显式组织视图排除已归档Agent并复用组织write可见性，未有组织记录按未分组/未置顶处理且不写记录；不带组织参数时保持旧授权列表read可见性和排序。游标绑定主体、查询和可见组织revision，组织变更需从首页重新读取；默认旧游标不因个人组织修改失效。全量内存扫描沿用现有grant列表实现，不宣称已改为数据库有界keyset扫描。

生命周期写结果未知时，使用 `changes/lookup` 按原action/key查询提交事实，不以当前GrantView状态猜测原写成功。每次查询仍需本人/租户/空间及当前permission，不能凭历史回执绕过撤权；404不证明写请求从未发生，DB故障明确不同于404。当前仅支持disable/enable/revoke/expiry；rename/组合patch继续保留既有合同，不把该lookup冒充所有mutation查询。

revoked 和 expired 为该 grant 的终态；PATCH 不能把它们恢复 active。期限修改（never ↔ duration 或重新选择限时）仅能在尚未过期的 active/disabled grant 上由人类确认并 CAS；期限成功修改从修改时刻计时，已过期不允许抢先续期。轮转原子更新凭据和 revision，保留 grant_id/agent_id/expiry_mode/expires_at/expiry_changed_at，不延长授权。disabled 可由本人重新启用，但仍须未到期且 revision 匹配。所有新身份从签发起绑定唯一 restriction profile，不能通过 PATCH 清除。

### 4.2 动作与强制限制

| actions 值 | 允许 | 明确拒绝 |
| --- | --- | --- |
| files.read | 授权目录的列表/元数据/已授权固定版本读取，精确搜索 | 跨前缀、跨连接、跨空间；内容分析与非授权导出 |
| labels.suggest | 提交该范围文件的 suggested 标签 | manual/rule 标签写入、采纳、覆盖原标签、改策略 |
| inbox.submit | 新产物进入本 grant 的隔离 staging，提交固定结果 | 指定任意正式目录、覆盖现有文件、移动/删除/直接归档 |

新授权绑定现有 Agent 身份并增加 `restriction_profile=personal_workspace_v1`；它不是仅用于 UI 的说明。新身份基础 scope 不授予宽泛 write/delete/import/export。所有 HTTP/SDK/MCP/worker 入口经过同一个 action 交集裁决：用户权限 ∩ 旧基础权限/新动作通道 ∩ grant 空间连接目录 ∩ 有效期/禁用/撤销 ∩ 当前操作用途。默认拒绝未登记动作，不能从旧 endpoint 绕回宽 scope。

读取版本、目录变更、上传分片、complete/publish、后台 job 逐步复检；不能仅提交时验一次。取消正在进行的写入只承诺阻止尚未发布部分；已保存版本保留审计，不谎称撤销回滚。

PATCH 扩大范围/增加动作/延长期限须人类重新确认；缩小也以 revision 并发控制。服务器使旧批准/待发布任务失效或进入可恢复阻断。旧 Agent 在页面标记“旧版授权”，不自动迁移、更不能自动缩权导致已授权服务失效；换成新 grant 是单独用户动作。

### 4.2.1 S2 跨入口映射决定：新写动作走专用 HTTP

明确选择：S2 不新增 MCP 收件/标签写工具，不将旧 `create_upload` 偷换成收件创建。新 grant 的 files.read 可用既有受权 MCP/HTTP；labels.suggest 和 inbox.submit 使用本节专用 HTTP 合同。SDK 薄封装同一应用服务。MCP capabilities 必须返回按身份过滤的动作说明与 HTTP 方法/路径/字段 schema/恢复说明，tools/list 只列实际已注册工具；不能把 HTTP action 宣称为 MCP tool。客户端没有 HTTP 字节/JSON 请求能力时明确 unsupported，不能退回宽 write。下表路径统一以 `/api/v1` 为前缀。

| 身份/动作 | MCP 入口 | HTTP 与返回 | 拒绝与恢复 |
| --- | --- | --- | --- |
| 旧身份且已有 write | 既有 create_upload、complete_upload、operation_get | 既有普通上传分片协议 | 保持旧语义，不自动创建 submission，不自动迁移成新 grant |
| 新 grant / files.read | 既有受权 list/files/stat/search；按实际注册发现 | 固定版本 content GET，当前范围内 metadata/query | 越界拒绝；新 grant 不因 read 获得 staging 内容读权 |
| 新 grant / inbox.submit 创建 | **create_upload 拒绝 forbidden**；无隐式收件工具 | POST /inbox-submissions：name、mime_type、size_bytes、sha256、可选caller/task_ref；Idempotency-Key；201 返回 submission_id、revision、operation_id、purpose=inbox_staging、part_size、part_path | 不接受 space_id/path/file_id/expected_version 或 owner/grant 覆盖参数；目录与连接由认证 grant 推导，staging由服务端生成 |
| 新 grant / inbox.submit 续传 | operation_get、complete_upload 仅允许本 grant 的 inbox_staging operation | PUT 返回的 part_path；同一 Bearer/context；GET /inbox-submissions/{id} 回读提交状态 | operation_get 仅返回自身操作安全元数据/received_parts，不扩展 files.read；无此purpose或他人operation拒绝；complete复检权限后关联固定版本与submitted |
| 新 grant / labels.suggest | 无新增 MCP 写工具，旧标签写工具不能代理 | POST /files/{id}/label-suggestions：version_id、labels[]，Idempotency-Key；200 返回 suggestion_id/revision/agent_id/固定版本/labels/state=pending | 无指定actor/manual/rule/采纳权限；同键同摘要重放同建议，不同摘要409；未知结果重放原POST或GET该文件自身建议并匹配原键 |
| 人类审核 | 不授予 Agent 审核工具 | submission decision/archive、suggestion decision 使用原表合同 | 即使同时具备三个新动作也不得自批、归档、覆盖正式文件 |

创建收件的幂等域为 instance+tenant+authenticated grant+action+key；POST与lookup引用同一记录，lookup只接原键，同键不同摘要409。分配 submission、原 operation 和 staging身份必须原子持久化；创建响应丢失用 POST /inbox-submissions/lookup 回读，不调用旧create_upload。lookup返回与创建相同的非秘密传输参数和原身份，无新对象。完成响应丢失查询原operation及submission，已submitted不再创建或发布第二版本；未完成只补received_parts之外的分片。禁用/到期/撤销后写与complete拒绝，安全元数据回查也不得绕过当前身份认证；失效凭据改由人类管理面回查，不给失效token例外。

标签建议重放响应包含原建议ID和状态，GET自身建议按服务端记录的创建键匹配（新增响应字段 `creation_key`，仅人类与原提交Agent可见，长度/字符限制同幂等键；客户端不得放秘密）。整个通道不授予内容读取；只有同时具有files.read才可读固定正文。仅inbox.submit也可以回读自己submission/operation的安全状态，但不能读取发布后正文或他人收件。

新 grant 的普通 uploads/create_upload、旧宽标签写、归档/移动/删除入口一律拒绝，不按参数猜意图转换。共享part/complete入口根据持久化的grant_id+purpose判定，不信任客户端声明purpose。S2 B07负责收件通道，B16负责标签通道，B05负责全入口拒绝；B12.2验证收件+读取最小链，完整三动作由B16/F23覆盖。新HTTP合同是规划，未实现前发现必须标未就绪，不能通过旧入口冒充支持。

### 4.3 个人 Agent 组织合同（新增）

分组和置顶是服务端个人组织数据，不是浏览器偏好，也不是授权策略。支持本人可管理的既有 agent_id（含旧版 Agent），不要求将旧 Agent 自动迁移为 grant。scope 为 instance+tenant+authenticated human user；body 中 owner/user/tenant 不可覆盖身份。不同空间的视图仍按可见性过滤，分组不共享给其他人。

| HTTP | 请求 | data / 约束 |
| --- | --- | --- |
| GET /agent-groups | cursor、limit | GroupView 分页；仅本人，空分组可见，不泄漏不可见成员计数 |
| POST /agent-groups | name（trim 后 1–80 字符），Idempotency-Key | 201 GroupView；同 owner/tenant 下规范化名称唯一，重复 409 name_conflict |
| GET /agent-groups/{id} | 无 | GroupView；不可见 404 |
| PATCH /agent-groups/{id} | expected_revision、name | GroupView；冲突 409 revision_conflict，保留编辑草稿 |
| DELETE /agent-groups/{id} | expected_revision，Idempotency-Key | 同事务解绑成员并写 tombstone；200 group_id/deleted；原键重放相同结果 |
| GET /agent-organizations | space_id 可选、group_id 或 ungrouped、pinned、cursor、limit | OrganizationView 分页；同一权限快照下按 pinned 降序、agent_id 升序稳定排序 |
| GET /agent-organizations/{agent_id} | 无 | 本人 OrganizationView；没有存储行时返回虚拟 revision=0、group_id=null、pinned=false |
| PATCH /agent-organizations/{agent_id} | expected_revision、group_id（null=解除）、pinned 可选，Idempotency-Key | OrganizationView；首次 revision=0 以唯一键 CAS 插入，不能覆盖另一设备创建 |

GroupView：`group_id, name, revision, created_at, updated_at`。OrganizationView：`agent_id, group_id:null|string, pinned:boolean, revision, updated_at`。组织列表与 grant 列表可按 agent_id 关联，grant 的授权 revision 不随组织变化而变，组织不得修改文件权限、scope、期限或凭据。合法可见 Agent 缺管理动作 403，跨用户/租户或 group 不可见 404；已被删除的本人 group 用于重新归组返回 409 revision_conflict，客户端重读。

删除组与归组争用相同组 revision/fencing：不产生悬空 group_id；解绑时每个受影响组织 revision 加一并保留 pinned，删除绝不 revoke Agent。并发重命名/置顶/归组仅一个 CAS 成功；另一设备刷新后读回服务器事实，不用 localStorage “最后写入赢”。分页游标绑定组织修订代次；删除/移动导致旧 cursor 失效 410，重查不漏/重复成员。所有变更遵循幂等规则且有非秘密审计；管理动作对 Agent bearer 全部拒绝。

### 4.4 调用记录与接入配置

B36/B37当前实现：只采集已认证workspace Agent的HTTP handler/MCP tool调用，服务端request ID与实际协议；不回填audit，不保存参数、凭据或响应正文。GET agent-invocations仅human当前owner/tenant/scope读取，撤销grant后owner历史仍可读，逐条核当前绑定与权限。分页coverage=best_effort、total=null，缺记录不等未调用；resource_refs=[]、operation_ref省略。时间参数RFC3339含时区，以真实时刻开区间比较；默认50/max200、候选1000预算、稳定ID序、cursor绑定筛选/权限范围/投影变化（包括finish）。当前记录仅unknown/denied，finished可空；不因HTTP成功宣称业务成功。UI、资源调用覆盖、资源/业务操作关联、保留规模仍待交付。

InvocationView：`invocation_id, agent_id, grant_id?, protocol, action, resource_refs[], operation_ref?, result, error_code?, started_at, finished_at?, request_id`。result=succeeded/partial/failed/denied/unknown；来源由已认证 Principal 写入，caller/task_ref 不参与身份认定。记录指向已有 audit/operation，禁止把记录当第二个执行状态机。配置面基于可发现的协议/服务地址和公开 capabilities 生成 MCP/API 示例，使用占位凭据；secret 仅一次手动复制，不写浏览器持久化。未安装 CLI 的 MCP 客户端通过 tools/list、inputSchema 和可读资源获得动作/恢复参数；本机路径不作为远端文件上传来源。显式 checks 仅验证配置/受权可读性，不能证明回存成功，也不主动派发外部 Agent 任务。正式页不提供“执行演示调用”，实际外部客户端发起的调用才进入记录；进度区复用 TaskProjection 的原 kind+ID 和持久结果，不使用“本次会话”内存任务号。

B65/B66增量半开区间使用 `[started_at_or_after, started_before)`；旧 `started_after` 继续严格排除下界，不能把旧名称解释为包含。两个下界参数互斥（即使同一时刻也400），参数必须单值有效RFC3339Nano；提供两界时起点须严格早于终点（相等/逆序400），可只提供任一边界，含时区按瞬间比较至纳秒。半开cursor使用独立query domain，不能与旧开区间互换；旧请求cursor绑定编码保留。B67/B68新增可选 `order=started_desc`：按started_at真实瞬间倒序（纳秒精度），同瞬间按invocation_id升序；在当前授权且有界的候选投影上排序后再分页，不用数据库含offset时间字符串排序。不传order保持B37原ID升序与旧cursor绑定。空/未知/重复order均400；排序纳入query domain，旧/新模式cursor互换410，任何可见投影变化仍令旧cursor失效。1000候选预算不因筛选窄范围或排序而放宽。

F80/F80.1正式页日期选择器现支持UTC或明确显示名称的本机时区：结束日期转次日零点并转UTC，按日历推进而非固定加24小时，使用包含下界的新参数；重复午夜取较早瞬间，不存在/调整午夜拒绝应用。真实浏览器覆盖纽约23h/25h与圣保罗跳过午夜拒绝。任意IANA时区选择仍未交付；started_at+stable ID排序已由B67/B68 owner提供，F81/F81.1正式Web默认旧ID顺序、可显式选择开始时间倒序；切换清游标并保留时间/result，旧服务拒绝时不自动退回更宽查询。真实三条调用观察验证服务端与UI一致顺序，跨页/tie由B68证据覆盖。owner半开时刻合同与该有限日期UI验收分别记录。授权状态和执行结果筛选可存本机偏好；Agent 选择、查询文本、时间仅会话保存。

## 5. 产物收件与人工决定（新增）

| HTTP | 请求 | data / 行为 |
| --- | --- | --- |
| POST /inbox-submissions | grant 身份推导来源；name、mime_type、size_bytes、sha256、可选 caller/task_ref（沿用短标识校验） | 201：submission_id、revision、原 upload operation_id、purpose=inbox_staging、上传限制 |
| POST /inbox-submissions/lookup | 原 idempotency_key | 同一 submission/operation，禁止重复创建 |
| GET /inbox-submissions | space_id、state、agent_id、cursor、limit | 固定产物引用、提交者、状态；人类可见所属范围，Agent 仅自身 |
| GET /inbox-submissions/{id} | 无 | SubmissionView + 上传/验收/归档分开的事实 |
| POST /inbox-submissions/{id}/decision | expected_revision、version_id、decision:accept/reject、可选 reason(≤500 字) | 人类专属决定；不自动移动、执行或删除 |
| POST /inbox-submissions/{id}/archive | expected_revision、version_id、target_space_id、target_prefix | 202：调用既有 archive 服务，返回原归档 operation_ref 与必返可空 file_ref（尚未发布时为 null）；完成后通过原 operation/submission 回读目标固定引用；同名默认冲突 |

分片与 complete 复用既有 operations 协议；服务端只对属于该 grant 的 purpose=inbox_staging operation 开放。完成后固定 file/version，状态变 submitted，不能由 Agent 自设 accepted。拒绝后不物理删除，受 retention 管理；归档后记录目标引用，来源与审核事实保留。

SubmissionView：`submission_id, revision, space_id, agent_id?, grant_id?, source_kind, source_claim?, file_ref?, version_id?, upload_operation_ref, state, decision?, archive_operation_ref?, archived_file_ref?, created_at, updated_at`。state=uploading/submitted/accepted/rejected/archiving/archived；失败使用 owner operation 的状态与 retry 信息，不抹去最后持久阶段。

`version_id`来自submission已持久绑定的原发布版本，uploading尚无固定版本时省略，后续accepted/archived仍保留原版本，不从File当前版本推断。前端固定验收与预览需同时使用file_ref/version_id；旧服务未返回时不得回退到最新版本。该metadata不授予正文读取权限。

用户自己的手机/网页导入复用原上传与收件能力；若进入统一 SubmissionView，source_kind=human_import 且无伪 Agent。归档不等于 Scaena/Pinax 等域采纳。接收或归档超时分别查询原 submission/operation，只有未决步骤可重试。

## 6. 文档、属性、资源与收藏（新增或复用）

| HTTP | 请求与响应合同 | 约束 |
| --- | --- | --- |
| GET /files/{id}/document-capability | file_id、version_id、editable、reason、max_bytes | 当前主体/版本的显式编辑能力；默认关闭，不从 read 推断 write |
| PUT /files/{id}/document | base_version_id、text；Idempotency-Key；返回 file_id/version_id/file_ref、operation_id、status | 已实现B17，人类 write/overwrite；UTF-8 Markdown/text ≤2 MiB、空正文合法；复用上传/版本发布CAS，冲突409，不修改旧版 |
| POST /files/{id}/document/lookup | idempotency_key；返回 found 与 nullable save | 回查当前主体/租户/目标的原operation，不重新发布 |
| GET /files/{id}/properties | version_id；返回 storage_kind、file_ref、definition_revision、values、parse_warnings | Markdown 来自指定版本 frontmatter；其他文件为 sidecar 投影；禁止从当前版补旧版 |
| PATCH /files/{id}/properties | base_version_id、expected_revision、typed updates/remove_keys | Server 验证类型并保存；Markdown 新版本，sidecar 新修订；禁止客户端静默转换 |
| GET/PUT /spaces/{id}/property-definitions | GET 当前定义；PUT expected_revision+definitions | 人类管理；稳定 key、display_name、group、type:text/number/date/checkbox/list/reference；key 改名/类型迁移先建计划 |
| GET /files/{id}/links | version_id、cursor、limit | 只返回当前有权看到的文件链接/反链及来源版本；投影可重建，不是 Pinax canonical 图谱 |
| GET/POST /favorites | space_id + 分页；POST target_kind:file/directory/tag/collection、稳定 target_ref | 个人 owner 下幂等；目录要绑定连接/root，不是裸路径；不授予额外权限 |
| DELETE /favorites/{id} | expected_revision | 只删收藏绑定，不删文件；幂等重复返回删除结果 |
| GET/POST /resource-descriptors | space_id + 分页；POST file_ref 或 collection_snapshot_id、kind、display_name | kind=skill/template/reference_pack；描述与固定版本绑定；不保存可执行凭据 |
| GET /resource-descriptors/{id} | 无 | 原文件引用、descriptor_revision、checks、owner_handoff、observed_at |
| POST /resource-descriptors/{id}/checks | expected_revision、file_ref | 只读格式/安全规则检查；返回 receipt/check_state，绝不安装或执行 |

资源资料包继续用集合快照，不创建第三种文件容器。promptrepo/Template Registry 的模板 canonical contract、Pinax 笔记业务及 Manju 内容索引各归原 owner。本文 references 只持稳定标识，目标不支持时显示 unsupported。

properties/labels 的字典管理若既有组织服务能表达则复用；需要新增时仅在同一组织 service/GORM 模型内扩展。范围写入、标签合并、字段 key/类型迁移走预览/确认/逐项结果，不提供无界批处理 endpoint。普通正文可编辑；项目配置、policy 或其他结构化项目资产必须通过其 CLI/application service，不由知识库编辑器绕过。

知识库列表复用 file-queries + MIME/路径筛选，Markdown 新文档复用 uploads。资源选择进入 Agent 上下文时固定 file/version，校验覆盖该版本；保存模板既不授予 Agent 使用许可，也不代表分析/费用批准。

### 6.0f 固定二进制版本 sidecar 属性（B57/B58，2026-10-06）

GET `/api/v1/files/{id}/properties?version_id=<fixed-version>`只接受一个version_id，返回file_id/version_id/definition_revision/revision/storage_kind=sidecar/values/unknown_keys。当前human与固定文件/版本权限校验，未知历史key仅返回key不返回未验证raw值，存储仍原样保留；引用值须本instance固定version当前可读，隐藏引用拒整view。新版本不自动继承旧值，Markdown/文本/配置暂不支持；二进制支持pdf/png/jpg/jpeg/gif/webp/mp3/wav/mp4/webm，不修改provider字节。

PATCH同路径required closed `{base_version_id,definition_revision,expected_revision,updates,remove_keys}`，256KiB、单Idempotency-Key、无query。当前head=baseversion、字典和属性双revision CAS，六类型按字典验证、引用重新授权；updates重复key（含转义等价）/null/无效UTF8/孤立surrogate拒，不lossy修复；remove_keys显式删除已知key，未知key不隐式清理。每值16KiB/整体256KiB/200keys，text解码8192bytes、list100个text每1024bytes、number精确abs≤2^53-1与指数[-308,308]、date真实YYYY-MM-DD、checkbox bool、reference固定ref；schema只约束union shape，最终预算/类型/授权归app。

写入默认关闭：服务启动时显式使用 `drivebridge-server serve --property-editing`（与既有实例必要配置一起使用）才启用；应用层Config.PropertyEditing同样默认false。关闭后GET和POST `.../properties/lookup`仍可当前授权读取。lookup closed `{version_id,idempotency_key}` 2KiB，返回file_id/version_id/request_digest/result_revision/committed_at原事实，不拿current猜成功。值与WorkspaceRequest同事务；历史无原receipt不补造，404非未提交。原摘要为Go JSON `{file_id,request:{base_version_id,definition_revision,expected_revision,updates,remove_keys}}`，updates按key排序/原值合法JSON压缩与Go转义，不可拿未经匹配的JS序列化摘要冒充。HTTP读取无业务audit/usage写。Web值编辑、Markdown无损frontmatter、旧投影迁移另验；D03未证仍不允许Pinax绑定或旁路。

### 6.0e 属性字典（B54/B55，2026-10-06）

GET `/api/v1/spaces/{id}/property-definitions`返回 `{space_id,revision,definitions}`，无字典为virtual revision0+[]，不初始化。人类当前owner/tenant/scope/root read；最多200定义、按key排序，不可用/坏数据不冒空数组。旧定义无聚合revision头返回dependency_unavailable，须独立迁移，不自动赋号。

PUT同路径要求单Idempotency-Key、无query、256KiB closed `{expected_revision,definitions}`，每项四字段必填 `{key,display_name,group,type}`；key小写字母开头与数字下划线≤64，display_name非空≤120Unicode字符，group≤80，两者无控制字符；type=text/number/date/checkbox/list/reference。当前human manage，完整列表CAS，新增/显示名/分组修改可用，删除/稳定key/type变更422 unsupported_action，留显式迁移。head/rows/原receipt原子，不写File/Version/frontmatter/sidecar值。

PUT返回 `{space_id,request_digest,result_revision,committed_at}` 原事实；POST `.../lookup`接closed `{idempotency_key}` 2KiB仅原receipt，当前read，缺404非未提交、DB500；GET/lookup无业务audit/usage写。digest为Go JSON `{space_id,request:{expected_revision,definitions}}`，定义字段按key/display_name/group/type顺序，保数组序及Go转义。当前字典不代历史回执，同key同请求回原事实不重写，异请求409，旧rev新key409。正式Web、文件属性值CRUD和迁移仍分别验收。

### 6.0d 集合快照原创建回执（B53，2026-10-06）

POST `/api/v1/file-collections/{id}/snapshots/creations` 接受closed `{expected_revision}`（安全正整数），1KiB、无query、单Idempotency-Key，人类当前collection read/write；发布前重新核捕获的File/Version/path/权限，header+entries+WorkspaceRequest同事务。200 data `{collection_id,snapshot_id,request_digest,collection_revision,committed_at}`，digest按Go JSON字段collection_id/expected_revision顺序SHA256。原key同请求只回原snapshot，禁止再次枚举；异请求409，新key旧rev冲突。

POST `.../snapshots/creations/lookup`仅closed `{idempotency_key}`，2KiB无query，当前human/collection read；原请求事实只读，无entry/body/业务audit/usage写。后续集合改版或创建新快照不改变原结果；缺receipt404不证明未提交，不通过最新snapshot补造。读原snapshot仍用GET `/collection-snapshots/{id}`，每项重新鉴权/旧version固定，历史提交不证明当前所有条目可见。初始rule枚举不是全库原子快照，页面须说明冻结提交阶段观察而非精确预览时刻。旧POST `/.../snapshots`与SDK保持无key合同，旧操作没有新receipt。Web消费另验。

### 6.0h 历史快照索引（B61/B62，2026-10-06）

GET `/api/v1/file-collections/{id}/snapshots`为human当前owner/tenant/space/collection read下的header-only观察，禁止query（包括limit/cursor），no-store且无业务audit/usage写。200 data为 `{collection_id,items,observed_at}`，items每项仅 `{snapshot_id,collection_revision,created_at}`，按created_at DESC/id DESC。最多1000，1001探测后429拒整次查询，不返回截断列表或伪分页。依赖错误500、坏header503，不冒空历史；空列表仍鉴权。

不查成员/文件/正文/原receipt，不输出历史或当前条目计数。legacy和keyed快照均可出现，但列表不是原提交事实；缺项不证明未创建、出现不保证成员当前可读。打开单个快照仍走现有ReadSnapshot逐成员重新授权，原创建恢复仍用原key lookup。相同路径旧POST创建保持兼容。Go生成SnapshotIndexEnvelope已可供客户端消费，历史页面另验。

### 6.0c 集合显式成员编辑基线（B52，2026-10-06）

GET `/api/v1/file-collections/{id}/members/decisions`，无query，人类当前集合read/write和逐file读取权限；200为 `{collection_id,revision,items:[{file_id,space_id,path,decision}]}`。items是完整include/exclude决策，不执行rule，不代表可见动态成员或固定snapshot；任一显式对象隐藏/回收/purge/staging/跨scope拒整表，绝不静默丢项形成可保存表单。最多10000条、JSON item累计1MiB-2048字节，超限429；坏持久决策503，数据库故障500，首尾集合改版query_changed。只读、无业务audit/usage写；保存仍通过B51当前权限/CAS再次核对。隐藏项治理与UI编辑另验。

### 6.0b 集合成员修改原回执（B51，2026-10-06）

新增人类POST `/api/v1/file-collections/{id}/members/changes`，单个Idempotency-Key，closed body `{expected_revision,include,exclude}` 三字段必填，数组不可null，合计≤10000、无重复/交集、成员48位小写hex；请求1MiB、无query。复用旧SetCollectionMembers的事务内准入/CAS/重建，receipt同事务。响应200 `{collection_id,request_digest,result_revision,committed_at}` 是原提交事实，无当前成员/正文/名称。digest为Go JSON对象collection_id与request（expected_revision/include/exclude按此序）SHA256，保数组顺序。原key同body重放只回原receipt，不做第二次修改；异body/目标409，新key旧rev409。旧PUT `/.../members` 保原无key合同，不造历史receipt。

POST `.../members/changes/lookup`仅接closed `{idempotency_key}`，2KiB无query、当前human/owner/tenant/collection scope/read。只读原事实、不读旧成员、不写业务audit/usage。后续成员变更或原文件trash不改变原receipt，也不证明目前仍可读；404不证明从未提交，500依赖错误不能冒不存在。没有原key时禁止以当前集合或revision猜成功；UI显式决策读取与编辑另验。

### 6.0a 集合创建原回执（B50，2026-10-06）

POST `/api/v1/file-collections/lookup` 接收 closed `{idempotency_key}`（非空ASCII可见字符、最多200），无query，2KiB body；人类session/CSRF及当前owner/tenant/space/root read。200 data含 `collection_id,space_id,request_digest,result_revision:1,committed_at`，只说明原创建事实，不含名称、规则、成员或当前revision。新human manual创建时在同事务写WorkspaceRequest；创建/receipt/返回计数任一失败整体回滚。成员revision增长不改变原创建回执，创建字段漂移409。旧集合、旧Agent创建、rule无对应receipt时404，绝不回填；404不证明未提交，不允许自动换key重建。当前失权拒绝，数据库故障500。查询不执行集合rule、不读成员/正文、不做audit/usage业务写。

旧POST `/file-collections`成功仍200/原FileCollectionView，增加128KiB closed顶层JSON、拒重复/未知字段及query、单个Idempotency-Key；合法旧Agent private/rule链保持。其返回当前view不冒原receipt。新receipt仅manual human，不代表rule输入完整校验或全F09写入已完成；正式Web创建消费另验。request_digest沿原Go fingerprint字段顺序 `space,name,kind,scope,rule`、JSON转义及SHA-256，不可用其他客户端摘要冒充。

### 6.0g 规则集合创建准入增量（B59，2026-10-06）

上述B50“仅manual human回执”为历史阶段：现在新human rule也同事务保存collection.create原receipt，旧行/Agent不补造。POST `/file-collections`仍返回当前集合view，原事实须POST `/file-collections/lookup`；其digest保旧字段顺序与原rule完整字符串（包括内部空白），不可由归一化后的规则反算。lookup不执行规则、不读旧成员，仍重新鉴权，后续member修订不改变原result_revision1。

创建和读取共用严格规则校验：rule≤64KiB、JSON闭输入、嵌套重复/未知/null/无效Unicode拒；mode=exact，space_ids可省略或仅当前集合space，cursor只空，limit省略或0..200（仅兼容页大小，集合仍完整有界枚举）。最多64条件、最多1排序，不支持字段/操作组合直接invalid_query，不静默忽略。path_prefix支持prefix/contains非空文本；mime_type/caller/task_ref仅eq文本；三个labels namespace支持eq/ne/contains非空文本；updated_at支持gt/gte/lt/lte+RFC3339；size_bytes支持gt/gte/lt/lte+非负安全整数；trashed仅eq false。排序当前支持updated_at/name/path_prefix/size_bytes、asc/desc。B59.1已修size游标为int64大小与ID同方向续页，205条跨页升降序及同size tie-break验证通过；旧错误timestamp大小游标返回query_changed，请从新查询开始，不复用其续页位置。坏旧规则读取失败，不自动改写或回退全空间。B60已生成CreateFileCollectionRequest、CollectionRule/Filter/Sort与FileCollectionViewEnvelope，POST现有路由保持200/current view和legacy Agent private兼容；原receipt独立lookup。manual旧请求携带rule会被忽略（含null），新消费者应省略；rule创建必须typed对象。Schema只校结构和组合，不替代空间身份、原字节/重复键/Unicode/预算校验；原raw identity需原样保留。正式规则创建UI尚待，不宣布完整集合管理完成。

### 6.0 普通文档保存实施状态（2026-10-05）

B17已实现以上三条文档路由，Go DTO位于 `server/internal/contract/document.go`，生成类型与fixture沿用既有生成器。服务端 `--document-editing` 默认关闭，只有显式启用及当前文件/主体权限满足才可写；扩展名限定 `.md/.markdown/.txt`，无media_type覆盖字段，Agent不能借人类编辑接口写入。实现复用CreateUpload/PutPart/Complete；同键核对主体、目标、base_version和正文digest，成功重放原receipt；确定性CAS冲突通过原Cancel释放预留额度。lookup在写入口关闭后仍允许已授权主体安全回读原结果。

Web当前只在知识库主窗格接入：内存草稿、差异确认、原键恢复；Web Locks保护跨标签恢复身份，不能持久化正文或token。冲突与未知结果不自动换键覆盖；完整合并/rebase界面尚未实现。本接口不证明本机Pinax受管保护已完成，D03独立阻断旁路后才允许绑定。

定向证据：`temp/integration-test-runs/20261004T190147Z-5d3af8fd/`（TestDocument）、`20261004T192533Z-e93c98c7/`与`20261004T192535Z-7db57236/`（生成/39schema）、`20261004T192628Z-ab0bdc49/`（真实fixture浏览器保存/版本/双tab冲突）。其余表项仍按各任务实际状态验收，不据文档存在推断已实现。

### 6.1 Pinax 阅读与本机门面交接（2026-10-01）

用户已确认 Electron 内置 Pinax 阅读空间。普通 Markdown 仍使用本节 file-queries/document/properties/links；Pinax note 则经独立公开 reader API，不能伪装为 File/Version，不能在本服务重算 canonical 双链或直读 `.pinax`。F06/F07/F09/B09继续拥有公共组件与文件规则，新桌面change仅补外部适配和本机行为。

新增本机 `/v1/desktop` 与 `ReadingRef`/`ReadingState` 合同唯一真源见 [Electron 架构§4](../architecture/electron-desktop.md#4-本机门面与类型均待实现)。这不是新增云端 `/api/v1` 路由，首片不上传本机阅读状态；相同Favorite/CAS领域规则由同仓application复用，不能root/server各复制一套。Pinax类型由其catalog生成，本项目DTO只加来源包装。

对于显式受管范围，本节文档/属性/标签写、既有上传覆盖/移动/删除/归档/同步/worker发布均必须经共享guard，返回owner-required，不因旧write或Pinax故障放开。旧挂载不自动变受管，绑定前覆盖校验和降级要求见 [管理范围](../architecture/electron-desktop.md#5-管理范围与旧挂载兼容)。新入口均待实现；表中普通文件合同不因此被宣称已安全接入Pinax。

## 7. 计划、自动化、设备与助手边界

organization-plans 原合同保留：CreateOrganizationPlanRequest、ConfirmOrganizationPlanRequest 的 expected_revision/impact_digest、ExecuteOrganizationPlanRequest；实现中以 Go DTO 为准，新端不得猜字段。原计划 phase/逐项 status 不改名；UI stale 是对 plan_stale/单项 stale 的投影。

“调整计划”以新 idempotency key 产生替代预览，记录 supersedes_plan_id（可选加法字段）；旧确认不能迁到新计划。新 scope/context digest 为可选补充，但新工作台创建时必填验证。确认与执行分离，Agent 不可 confirm，不可更改 organization-policy；delete/overwrite/cross-space/permission/paid analysis 各有独立风险决策。

设备与离线首期消费可发现宿主能力；纯 Web 只显示可证实的浏览器缓存/前台传输能力，不新增虚假设备表。身份/配对不等于全盘权限，云端撤权不能召回已导出字节。automation-rules 复用既有应用合同；需要本机宿主时挂起而非伪完成。

内置文件助手保留 S5：仅文件上下文与建议/原计划引用，分析走 Manju/领域 owner、模型访问走已批准 runtime；不在 Server 新增通用 Agent 执行器。本次不宣称 assistant-sessions 路由已存在。S5 任务必须先固定 runtime/会话持久化合同，再开放 UI 入口；外部 Agent 主链不依赖该切片。

### 7.1 内置文件助手 S5 handoff 合同（B14.1）

S5 前置交接：助手能力不新建 runtime，也不在 Server 增加通用 Agent 执行器。会话与模型访问全部走已批准 runtime owner（Manju/领域 owner 负责分析；模型额度由该 owner 记账），Server 只保存可复核的引用，不保存正文或对话副本。助手可见上下文固定为：用户显式打开的文件固定版本引用、原计划/建议引用、用户在会话内显式附加的引用；不得后台扫描或隐式扩权。分析同意按会话显式记录（同意主体、时间、范围），预算在会话级封顶并可由用户随时中止，预算耗尽不得换账号或换 runtime 续跑。所有助手产物引用走既有 file/version/operation 身份与原 key 幂等。该需求保持未完成状态，本文仅固定合同边界；S5 开工时以本节为准，不允许绕过 runtime owner 自建会话或计费。

### 7.2 设备、离线与续接 S5 handoff 合同（B14.2）

S5 前置交接：本机与原生客户端只持有宿主可证实的能力与受权缓存（浏览器缓存/前台传输/宿主报告的离线副本），不伪造设备表或在线状态；Server 仍拥有文件任务、配对身份与凭据生命周期，客户端不得自签或缓存凭据。离线副本必须带版本与观察时间；云端撤权不召回已导出字节，但停止新授权与续传。续接（恢复阅读/传输/计划）一律回到原 operation/原 idempotency key，客户端不得凭本地状态重建身份。原生验收与 Web 降级分开报告：原生能力只在真实系统实测后启用，Web 侧明确显示不支持，不以浏览器视口模拟冒充原生。真实系统验证另报，不在本合同内勾选。

## 8. 错误、并发与恢复表

| HTTP / code | 触发 | 客户端恢复 |
| --- | --- | --- |
| 401 unauthorized | 浏览器 session 失效 | 清敏感缓存，重新登录；保留非秘密原请求身份 |
| 403 forbidden / approval_required | 当前动作未授权/未确认 | 显示范围和阻断原因；不得退回旧宽 scope |
| 404 not_found | 对象不存在或不可见 | 安全空态；不泄漏其他人的名称/总数 |
| 409 revision_conflict / version_conflict | stale 表单、旧正文版本 | 保留草稿，重新读最新并比较；不强制覆盖 |
| 409 plan_stale / policy_revoked | 源版本/策略/授权变化 | 查询原计划/逐项结果，重建预览并重审 |
| 409 idempotency_conflict | 同键不同请求 | 停止自动提交，显示原请求可查询入口 |
| 409 name_conflict / target_conflict | 收件归档/复制目标冲突 | 人类选新名称/新位置；不能自动覆盖 |
| 410 cursor_expired / snapshot_expired | 视图过期或授权修订变化 | 重新查询、清选择，确认完整新清单 |
| 413 quota_exceeded | 文件/空间额度不足 | 保留耐久进度；释放/更换目标后恢复原操作 |
| 422 unsupported_action | 未实现能力/未知动作 | 明确禁用及替代入口 |
| 429 throttled / budget_exceeded | 请求/策略限额 | 按 Retry-After 退避，保留原操作 |
| 503 dependency_unavailable / storage_disconnected | 依赖缺失/连接断开 | 提供修复入口，原任务等待，不新建 |
| 无响应 | 写结果未知 | 原键 lookup 或原 ID GET；禁止直接换键 |

本包新接口的限制 profile、secret_state 与接收决策错误使用上述稳定类型组合；确需新 code 时在 B01 统一注册并加入旧客户端 unknown-code 安全 fallback 测试。

## 9. 可执行验收与证据

B01–B16 在本 owner [tasks](../../openspec/changes/drivebridge-personal-workspace-contracts-v1/tasks.md) 管理。现有真实检查入口：

合同生成（Go DTO 唯一真源；personal-workspace 为默认 profile，desktop 另行冻结）：

```bash
make contracts       # web/scripts/generate-contract-types.mjs --profile all
make contracts-check # 生成漂移门 + schema 正负例 + desktop DTO 冻结 digest
go -C server run ./cmd/drivebridge-contracts fixtures --profile personal-workspace
```

从 `cli/drivebridge` 执行：

```bash
python3 scripts/check.py -- go -C server test ./internal/service ./sdk
go -C server vet ./...
(cd server && CGO_ENABLED=0 go build ./cmd/drivebridge-server)
openspec validate drivebridge-personal-workspace-contracts-v1 --strict --no-interactive
git diff --check
```

2026-10-10 B13 稳定门证据（均由根 runner 六件套）：service+sdk 全量 `20261010T160917Z-6e7c2c0a`；根模块全量 `20261010T160919Z-f1f92ae6`；server 全量 `20261010T161828Z-8b218bf1`；vet/build/CGO build 七项 `20261010T161840Z-ae2aa043`…`20261010T161855Z-5052f2a5` 全 passed；strict 与 `git diff --check` 通过。能力状态：本地 fixture/合同/SDK/MCP 已验；真实云账号、付费模型、原生客户端与发布部署未验，保持未完成。旧新版兼容由 `TestWorkspaceLegacyAgentDefaultsAndArrayContract`、`TestWorkspaceContractExisting*` 与 sdk `workspace_contract_test` 守护；降级演练由 `workspace_downgrade*_test` 固定（包含于全量证据），不认识 restriction 的旧 server 不能带有效新 grant 降级。

定向用例先行、稳定后全量的流程保持不变；2026-10-10 已按上表执行并留证。强制反例：同用户 A/B Agent 互读 staging、相邻前缀绕过、过期、旧接口绕权、撤权与 complete 竞争、secret 丢响应、双确认、cross-tenant 计数、迟到响应、属性冲突与私密链接。证据由根 runner 生成到 `temp/integration-test-runs/<run-id>/`；不记录 secret/正文/Provider payload。不得运行 race。
