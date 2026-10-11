# 查询分析与Agent有序管理（设计）

## 查询路径

精确目录/清单由事务库负责，容量/事件/分面由ClickHouse加速，DuckDB读取显式授权快照。文档/图片/音视频语义候选和证据由Manju提供。客户端只组合结果，不能在浏览器或适配器新建另一套索引。

查询使用限定dataset/field/operator/metric，禁止任意SQL。结果显示exact/discovery、generation、observed_at、coverage、partial、缺失来源和分页；Top-K只表示相关候选，不是全部匹配。按现行权限同时过滤明细、摘要、计数与facet。

总览可分析容量、增长、流向、恢复率、队列等待、失败原因、Agent活动和整理覆盖。logical/physical/reserved/cache/history/trash口径分开；有效流量不与观测流量相加；无账单时费用未知。缺少数据不是零。

## 虚拟整理优先

标签分manual/rule/suggested；人工确认不会被模型重跑覆盖。manual集合默认指文件当前版本，snapshot固定版本；rule集合保存版本化条件及人工纳入/排除。文件集合不替代Manju资产集合或Pinax笔记组织。

精确重复由已验证SHA-256判断，仅在授权可见范围分组；相似度和同名不是删除依据。所有移动、覆盖、删除、跨空间动作从查询结果形成固定计划后重新授权。

## Agent风险表

| 动作 | 默认规则 |
| --- | --- |
| 读取、统计、生成建议 | 已有read范围内执行 |
| 标签/集合修改 | 有人类预授权规则或本次确认 |
| 重命名/移动/复制 | 默认计划确认；明确范围/额度/期限规则可预授权 |
| 覆盖/软删除/跨空间外发/权限变化 | 单独确认，普通write不等价 |
| 永久删除/凭据管理 | 不进入自主整理面 |

计划冻结资源、版本、作用域、policy revision和影响摘要；执行时复查。stale不自动重算；partial只恢复未完成项；compensation是独立反向计划，后续版本冲突不能强行撤销。文件正文和模型建议不能提供权限。


### 虚拟组织实施状态（已实现，本地验证）

文件组织面已落地并由 `TestFPFileOrganization*` 本地验证（fixture，离线）：

- 数据模型（纯新增表，回滚仅删新表）：`file_label_assignments`（(owner,file_id,namespace,label) 唯一）、`file_collections`（manual/rule、scope、rule_revision、乐观 revision、创建幂等键）、`collection_members`（显式 include/exclude）、`collection_snapshots`/`collection_snapshot_entries`（钉 file/version）、`saved_file_queries`。
- 标签权威：manual 仅人类显式动作、rule 仅预授权规则、suggested 仅模型；Agent 写 manual/rule 返回 `label_namespace_forbidden`（403）且人工层不被重跑覆盖；`expected_revision` 过期为 `version_conflict`。
- 精确查询：`POST /api/v1/file-queries`——封闭词表（mime_type/path_prefix/size_bytes/updated_at/caller/task_ref/trashed/labels.<namespace>），keyset 分页（默认50最大200），游标 AEAD 密封绑定 owner+规范查询+catalogue revision（不一致 `query_changed` 409）；`total_count` 仅在授权范围内完整枚举时给值；trashed 默认不可见。
- 集合与快照：manual 集合跟随当前版本、快照钉捕获版本；成员失权（含 trash）在动态读与旧快照中同时过滤成员与计数，快照不扩权；过期成员写 `collection_revision_conflict`（409）。
- 保存查询：`GET/POST /api/v1/saved-searches` 只存允许字段，执行时重新授权。
- 重复候选：`mode=duplicates` 仅按已验证 SHA-256 在可读范围分组为 exact；名称相近摘要不同标 similar，未知摘要跳过；结果附"永不授权 purge"守卫说明。
- MCP 工具：`files_query`/`file_labels_set`/`file_collections_list`/`file_collection_create`/`collection_members_set`/`collection_snapshot`/`saved_searches_list`/`saved_search_create`；SDK 同名方法走同一应用服务。
- 加法索引：`files(owner, updated)` 支撑 catalogue revision 与 keyset 排序（design 允许的可回填新增索引）。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganization -count=1 -timeout 15m
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPFileOrganizationScaleQueryAndCollections -count=1 -timeout 15m
```

规模留白：quick（10000文件/1000成员）实测通过；20并发逐页 P95 在单写者 SQLite fixture（同步审计写入）为 5.82s，2s 生产目标绑定索引化 team catalogue（PostgreSQL），未在本地声明；full（1M/10000成员）可按上命令复跑。语义候选（Manju）与发布部署独立验收。


### 分析检索实施状态（已实现，本地验证）

分析面已落地并由 `TestFPAnalyticsSearch*` 本地验证（fixture，离线）：

- 合同：`POST /api/v1/analytics/queries`（封闭词表：dataset=file_current/transfer_facts/audit_facts/capacity_daily，filters≤32/dimensions≤3/sort≤3，各数据集固定 metric 集；权限范围由服务注入，SQL/路径 payload 一律 `invalid_query`）、`GET /api/v1/analytics/queries/{id}`、`POST /api/v1/analytics/snapshots`（人类显式导出授权；Agent 拒止；manifest 含 schema/generation/水位/字段/摘要，幂等键重放同 ID）、`POST /api/v1/analytics/snapshots/{id}/queries`（typed 校验先行，引擎缺失 `analytics_backend_unavailable` 503）。
- consistency：`live` 由事务库精确回答（backend=transactional，coverage/partial 事实）；`observed` 需分析后端部署，未部署时如实 503，绝不把事务数据冒充分析结果、不静默回退（spec 场景1）。
- 安全：Agent 只统计其绑定空间；聚合与钻取同范围一致；撤权即失读且计数不泄漏；跨 owner 各自范围零泄漏（spec 场景2）。
- DuckDB：可选固定版本引擎未安装——`analytics_backend_unavailable` 不自动安装/换后端，在线上传不受影响（spec 场景4）；快照查询的 SQL/路径 payload 在引擎接触前被 typed 校验拒绝。
- 数据模型：`analytics_snapshots` 纯新增表（幂等键唯一索引），回滚仅删新表。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPAnalyticsSearch -count=1 -timeout 5m
```

留白：100万文件/1000万事件、20并发 P95≤3秒、SQL基线/ClickHouse/DuckDB 三后端一致性与中文 golden 需真实分析后端部署，本机未验（分析后端未部署）；ClickHouse/ClickHouse 运维面不在本 owner。


### Agent有序整理实施状态（已实现，本地验证）

Agent整理面已落地并由 `TestFPAgentOrganization*` 本地验证（fixture，离线）：

- 合同：`POST /api/v1/organization-plans`（精确固定集合=快照或显式refs；typed 动作 add_label/move/trash/remove_label/restore；幂等键重放）、`GET /{id}`（revision+分页逐项影响+risk+等待原因）、`POST /{id}/confirm`（人类专属；绑定 expected_revision+impact_digest，错配 `plan_stale`；确认不扩权）、`POST /{id}/execute`（已确认或当前有效预授权才开执行；逐项复检权限/源版本/策略；漂移单项 `stale`）、`POST /{id}/compensations`（从已记录结果派生反向计划，不立即执行；后续修改 `compensation_conflict` 待人工）、`PUT/GET /api/v1/spaces/{id}/organization-policy`（人类专属预授权：动作/前缀/批次/字节限额/有效期+乐观修订）。
- 语义：未匹配预授权 `approval_required`（Agent 不能自批）；预授权失效 `policy_revoked` 停止未执行项保留完成项；policy_scoped 是路由事实非免鉴权，执行时仍逐项校验；覆盖/永久删除/凭据不在动作词表。
- 数据模型：`organization_policies`/`organization_plans`/`organization_plan_items` 纯新增表。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPAgentOrganization -count=1 -timeout 5m
```

留白：10000项计划、20并发规则变更与权限撤销竞态规模 fixture 本机未跑；补偿的 move 反向保持人工（当前树状态不可 blanket 反推）。

## 规格入口

- [虚拟组织](../../../openspec/changes/drivebridge-file-organization-v1/design.md)
- [分析检索](../../../openspec/changes/drivebridge-analytics-search-v1/design.md)
- [Agent整理](../../../openspec/changes/drivebridge-agent-organization-v1/design.md)
- [受限自动化](../../../openspec/changes/drivebridge-controlled-automation-v1/design.md)
