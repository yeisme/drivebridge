# 内容交接、交付与版本协作（设计）

## 版本讨论与私人备注实施状态（已实现，本地验证）

固定版本讨论与私人备注已落地并由 `TestFPVersionDiscussion*` 本地验证（fixture，离线）：

- `GET/POST /api/v1/files/{id}/threads`（创建幂等，默认 file 级锚点，可显式 version 锚点）、`POST /api/v1/threads/{id}/replies`（幂等）、`PATCH/DELETE /api/v1/threads-messages/{id}`（`expected_revision`，仅作者正文管理，删除保留审计占位）、`POST /api/v1/threads/{id}/{resolve,reopen}`（仅改线程状态，不动领域 review）、`GET/PUT /api/v1/files/{id}/private-note`（仅作者行+当前文件权限）、`POST /api/v1/files/{id}/private-note/publish-preview`（仅预览，不产生团队消息）。
- 正文纯文本 ≤10000 字符；版本锚点不随新版本漂移，原件清理后按权限保留讨论并标记 `anchor_version_unavailable`；解决线程不授予领域批准。
- MCP 工具 `file_threads_list/file_thread_create/thread_reply/thread_message_edit/thread_message_delete/thread_resolve/thread_reopen/private_note_get/private_note_set/private_note_publish_preview` 与 SDK 同名方法调用同一应用服务。
- 新错误码：`revision_conflict`/`anchor_version_unavailable`（409）、`comment_write_forbidden`（403）。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPVersionDiscussion -count=1
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPVersionDiscussion -count=1
```

回滚演练：`file_threads/thread_messages/private_notes/discussion_revisions` 为纯新增表；`TestFPVersionDiscussionSchemaAdditive` 已演练仅删除新增表后旧合同不受影响。规模验收：10000 条讨论（5000 线程+5000 回复）游标分页、20 并发编辑（1 胜 19 冲突）、双用户 private-note、XSS 纯文本、旧版本锚点、草稿恢复（revision_conflict 保留输入）均已达标（20260925T183610Z-14d50bc9）。

## 版本保护与保留计划实施状态（已实现，本地验证）

服务端版本保护与保留计划已落地并由 `TestFPRetentionGuards*` 本地验证（fixture，离线）：

- `POST /api/v1/retention/holds`（幂等键）、`GET /api/v1/retention/holds/{id}`、`POST /api/v1/retention/holds/{id}/release`、`POST /api/v1/retention/plans`（预览，Agent 可读）、`POST /api/v1/retention/plans/{id}/execute`（仅人类；MCP 不注册执行工具）。
- hold 绑定 file/version、purpose（fixed_delivery/legal_hold/transfer_window）、来源 share/operation 与 expires_at；来源被撤销/取消或到期即失去保护力；不承诺永久保存。
- purge 两阶段标记（`File.purge_started_at` 可空加法列）与 hold 创建互斥：已开始清理返回 `purge_in_progress`；活跃 hold 下 purge 返回 `version_protected` 并说明保护来源。
- 预览固定 policy revision；执行逐项重验 hold、引用与权限，`retention_plan_stale` 拒绝过期计划；共享 blob 仅在事务真源无其他引用时释放。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPRetentionGuards -count=1
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPRetentionGuards -count=1
```

回滚演练：`retention_holds/retention_policies/retention_plans/retention_items` 为纯新增表，`File.purge_started_at` 为可空加法列；`TestFPRetentionGuardsSchemaAdditive` 已演练仅删除新增表后旧 purge 合同不受影响。百万版本规模未在本机验证（见 5.2 证据 artifacts/scale-metrics.md）；真实后端与发布独立验收。

## 内容边界

DriveBridge拥有文件版本与读取授权，Manju拥有asset revision/derivation/index generation/evidence。固定binding连接instance/file/version/sha256与Manju项目修订。只有双边身份和资源权限均满足时，才返回内容摘要、locator和计数；身份不按邮箱或caller推导。

文档/图片理解沿用Manju，视频时序沿用Anatomia，音频证据消费Sonora公开合同。Anatomia独立ASR/OCR已经退役：视觉推断文字/对白不是ASR/OCR观测。音频缺匹配合同时明确unsupported并保留required交接，不在DriveBridge实现替代引擎。

内容处理默认关闭，用户可按空间/前缀/类型启用处理层级、预算和外发策略。新版本产生独立意图，历史回填是另一个有界任务。纯搜索不调用新模型；失败或预算耗尽不影响原文件读取。旧任务晚到不得挂到新字节或复活墓碑。

## 文件交付

固定分享复用Share/ShareItem并补服务生成manifest。成员、版本、大小和摘要全部核验且保留保护已建立后，才发布完整交付。缺项恢复原组；替换成员创建新清单revision。持续目录分享维持原语义。

accessible/opened/bytes_served/receiver_verified/receiver_confirmed分别记录；浏览器下载请求不是确认保存。DriveBridge确认不是Manju/Scaena/Eikona领域采纳。撤销未来读取不能召回已下载副本。

## 讨论与保留

Server持久化版本线程和私人备注；版本不暗中升级，评论解决不改变领域审核。私人备注同时要求作者身份和当前文件权限，公开需先预览目标团队并显式创建评论。

保留保护与purge在事务层互斥；活跃交付、在途传输和其他有效引用阻止清理。共享blob仍有引用时不能删除。分析库观察结果不能决定清理资格。


### 内容检索实施状态（已实现，本地验证）

内容检索面已落地并由 `TestFPContentRetrieval*` 本地验证（fixture+loopback Manju stub，离线）：

- 合同：`POST/GET /api/v1/content-bindings`（钉定 file/version ref+manju_connection_id+project_ref+purpose，幂等键重放同ID）、`GET /api/v1/content-bindings/{id}`（状态+原owner恢复链接）、`POST /{id}/process`（policy_revision 过期 version_conflict）、`PUT/GET /api/v1/spaces/{id}/content-policy`（人类专属，乐观修订）、`POST /api/v1/content-queries`（exact=受管事实；discovery=Manju typed 查询，逐项再授权+外来ref丢弃+missing 模态报告）。
- 语义：无启用策略 `content_policy_required`（默认不自动处理）；版本漂移 `content_index_stale`（绑定 stale，晚到结果不挂新版本）；未知模态 `modality_unsupported`（不换替代模型）；未映射连接 `identity_mapping_required`（不按邮箱/caller 猜测）；处理失败不影响原件可读。
- Manju 连接复用 creative connection 合同（kind=manju、人类配置、sealed secret）；Agent 可消费既有连接但禁策略扩权。
- 数据模型：`content_policies`/`content_bindings` 纯新增表。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPContentRetrieval -count=1 -timeout 5m
```

留白：四模态1000条证据 fixture、真实 Manju/Anatomia/Sonora 后端、模型质量评测独立验收未执行。


### 固定交付实施状态（已实现，本地验证）

固定交付面已落地并由 `TestFPFixedDelivery*` 本地验证（fixture，离线）：

- 合同：`POST /api/v1/shares/{id}/delivery-manifests`（人类分享管理权限+幂等键；从集合快照冻结成员与版本）、`GET .../{revision}`（逐项可读性/摘要核验/hold引用/完整标志）、`POST .../{revision}/publish`（人类专属；全部必需项 verified + retention_ready 才完成，缺项 `delivery_incomplete`、无保护 `retention_not_ready`）、`POST /api/v1/delivery-receipts`（回执分级）。
- 接收事实分级：accessible/opened/bytes_served/declared（自报保持声明级）与 receiver_verified/receiver_confirmed（需已认证接收方，匿名 `receiver_identity_required` 不冒充）；DriveBridge 确认不等于领域采纳。
- 保留保护依赖 retention-guards-v2 既有合同（活跃 hold 覆盖钉定版本即 retention_ready）；撤销先拒未来读取（share_unavailable），已传出事实保留。
- 数据模型：`delivery_manifests`/`delivery_manifest_items`/`delivery_receipts` 纯新增表。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPFixedDelivery -count=1 -timeout 5m
```

留白：10000成员规模 fixture、真实接收方联调独立验收未执行。

## 规格入口

- [内容绑定](../../../openspec/changes/drivebridge-content-retrieval-v1/design.md)
- [固定交付](../../../openspec/changes/drivebridge-fixed-delivery-v2/design.md)
- [版本讨论](../../../openspec/changes/drivebridge-version-discussion-v1/design.md)
- [保留保护](../../../openspec/changes/drivebridge-retention-guards-v2/design.md)
