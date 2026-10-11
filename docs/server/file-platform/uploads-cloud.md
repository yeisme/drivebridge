# 上传、云端管理与恢复（设计）

## 当前与目标

当前 `files.go` 使用8MiB上传分片、SHA-256验证和原Operation回读；`model.go`缺省MaxBytes为1GiB。浏览器 `api.ts` 在创建时生成随机键，Server完成阶段会拼装临时整文件。100GiB/万文件批次是本轮待实施目标，不能通过单独调大MaxBytes就声称支持。

## 新增上传闭环

枚举相对目录→创建持久批次→分页登记item→seal→每项原子配额预留→传输→完整摘要验证→后端读回→发布固定版本。上传前保存非秘密原键；创建/完成响应丢失先lookup/operation_get，不换键新建。

重开网页需重新授予文件句柄或选择同文件，hash不符拒绝。分阶段显示摘要、已耐久字节、验证、发布。pause保留数据；cancel只取消未完成项并释放其预留，完成文件不自动删除。

团队新会话将暂存置于受管对象存储，以有界读写和校验替代双份整文件落盘。旧8MiBAPI保留。后端分片独立映射：S3默认64MiB（100GiB为1600片）、OneDrive默认10MiB顺序块、Google Drive默认8MiB。Provider会话过期需先查原副作用；显式恢复可更新会话代次但不改变原业务意图。

官方依据：[S3分片上限](https://docs.aws.amazon.com/AmazonS3/latest/userguide/qfacts.html)、[OneDrive upload session](https://learn.microsoft.com/en-us/graph/api/driveitem-createuploadsession?view=graph-rest-1.0)、[Google Drive resumable upload](https://developers.google.com/workspace/drive/api/guides/manage-uploads)。后端兼容实现仍须各自验证，不能把S3官方限制直接当作所有兼容存储承诺。

## 云端目录

OneDrive/S3/Google Drive各自有服务端adapter；不切换多用户全局rclone配置。凭据由用户配置并绑定允许root，Agent只能在当前范围消费。外部entry观测与受管FileVersion分开；无可靠历史字节时只声称当前观测。

源修订变化时拒绝旧消费；Google原生文档先列举并提示export_required，不能伪装成普通文件下载。S3前缀移动是复制/验证/清理，不是原子目录改名。未提供原生变化流时只做预算扫描。

### 三类云连接器实施状态（已实现，本地验证）

统一云目录面已落地并由 `TestFPCloudConnectors*` 本地验证（fixture，离线）。默认关闭：`cloud_connectors` 配置未启用时新面一律 `unsupported`，旧路由与工具行为不变（`TestFPCloudConnectorsCapabilityGate`）。

- 根绑定与浏览：`POST /api/v1/connections/{id}/roots`（仅人类会话）、`GET /api/v1/connections/{id}/roots`、`GET /api/v1/connections/{id}/entries`（`root_id`+可选`parent_ref`，分页默认50最大200，游标不透明且绑定root/查询/身份，过期或跨查询 `cursor_expired` 410）、`POST /api/v1/connections/{id}/entries/stat`（`entry_ref`+`expected_revision`，漂移 `source_changed` 409）。
- 变化与扫描：`GET /api/v1/connections/{id}/changes`（OneDrive delta / Google changes；S3 无原生游标返回 `unsupported`，改走预算扫描）、`POST /api/v1/connections/{id}/scan`（幂等键绑定 connection+root+budget；partial 扫描永不推导删除，仅完整枚举对确认缺席写墓碑）。
- 导入：`POST /api/v1/connections/{id}/entries/import` 下载钉定观测、验证 SHA-256、发布受管副本并回写 `managed_ref` 与 `version_strength=sha256_verified`；S3 未版本化桶上的被超越修订返回 `version_unavailable` 409，Google 原生文档 `export_required` 422。
- 能力矩阵：`GET /api/v1/connections/{id}/cloud-capabilities` 逐 provider 独立申报 browse/transfer/historical_read/native_copy/soft_delete/conditional_write/change_cursor（S3 的 historical_read/soft_delete 依实测 bucket versioning）。
- MCP 工具：`cloud_entries`/`cloud_entry_stat`/`cloud_changes`/`cloud_scan`（Agent 需 connection 绑定；连接管理动作不注册给 Agent）；SDK 同名方法走同一应用服务。
- 数据模型：`cloud_roots`（root 绑定、generation 游标失效、扫描事实）与 `external_entries`（(root_id, provider_item_id) 唯一观测身份、tombstone、managed 链接列），均为纯新增表；回滚仅删两张新表（`TestFPCloudConnectorsSchemaAdditive` 演练）。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectors -count=1
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPCloudConnectorsScalePagination -count=1
```

真实后端目录、各 provider OAuth 与部署启用（`cloud_connectors` 开关）独立验收，fixture 不代替。


### 跨存储传输实施状态（已实现，本地验证）

跨存储复制/迁移面已落地并由 `TestFPTransferPlans*` 本地验证（fixture，离线）。默认关闭：`transfer_plans` 未启用时 `unsupported`。

- 数据模型（纯新增表）：`transfer_plans`（冻结来源集合+目标+冲突策略+乐观 revision+创建幂等键+cleanup 授权位）、`transfer_items`（钉定 source file/version/digest、逐项 copy operation、target ref、verification、cleanup_state、result_certainty）。
- 合同：`POST /api/v1/transfer-plans`（幂等键同键同指纹；来源为钉定 ref+摘要，漂移 `source_changed`）、`GET /api/v1/transfer-plans/{id}`（分页影响预览，计数为计划级事实）、`POST /{id}/execute`（`expected_revision` 过期 `plan_stale`；逐项走原 archive 合同，重放落原 operation；fail 策略占位目标 `target_conflict`，keep_both 走改名）、`POST /{id}/cleanup-source`（仅人类会话；只软删除仍匹配钉定版本的来源；缺口保留 `copied_source_pending`，已验证副本不回滚）、`POST /{id}/cancel`（完成项保留）。
- 跨空间传出是独立授权：`cross_space_confirmed` 显式确认，普通目标 write 不够；覆盖需 per-target expected_version+人工确认，不在计划词表（详见任务 1.2 注记）。
- MCP 工具：`transfer_plan`/`transfer_plan_get`/`transfer_execute`；cleanup-source 无 Agent 工具；SDK 同名方法同一应用服务。
- 修复两处潜伏死锁（Agent+改名路径+单连接 SQLite：事务内池化 allow 查询）——`allowOn(tx)` 事务绑定检查。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlans -count=1 -timeout 15m
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPTransferPlansScalePartialRecoverCancel -count=1 -timeout 60m
```

规模留白：quick=100 全维度过；10000 项 full 变体可复跑（单写者 fixture 逐项归档约4项/秒）；100GiB 真实云间流、真实后端与部署独立验收。

## 跨存储迁移

先固定source、target、version和冲突策略，逐项复制与校验，再由独立确认进行来源软删除。复制完成而来源删除失败显示copied_source_pending；重试不重复复制。来源或目标变更使原计划stale；补偿也需核对当前版本。

## 批次与恢复实施状态（已实现，本地验证）

上传批次与丢失响应恢复的应用合同已落地并由 `TestFPUploadRecovery*` 本地验证（fixture，离线）：

- `POST /api/v1/upload-batches`（幂等键同键同指纹复用）、`POST /api/v1/upload-batches/{id}/items`（每页≤500，拒绝绝对路径/`..`/大小写碰撞）、`POST /api/v1/upload-batches/{id}/seal`、`GET /api/v1/upload-batches/{id}/items`（游标分页，默认50最大200）、`POST /api/v1/upload-batches/{id}/{pause,resume,cancel}`、`POST /api/v1/operations/lookup`、`GET /api/v1/operations/{id}/session`。
- MCP 工具 `upload_batch_create/upload_batch_add_items/upload_batch_seal/upload_batch_get/upload_batch_pause/upload_batch_resume/upload_batch_cancel/operation_lookup/upload_session` 与 SDK 同名方法调用同一应用服务。
- 逐项调度复用既有上传合同（`POST /api/v1/uploads` 携 `batch_id`+`item_key`）：创建事务内原子预留，最多4文件并行（超出返回 `budget_exceeded`），`new_version` 策略必须 `file_id`+`expected_version`，`keep_both` 改名不失败。
- 批次绑定上传的完成走流式后端重组（本地单遍摘要+MIME 嗅探、整分片 Seek 回卷供签名、读回校验），不落第二份整文件；加密空间保留既有加密分帧组装路径。
- 新错误码：`batch_not_sealed`/`source_reselection_required`/`transfer_session_expired`（409）。

真实命令（本地 fixture 验证入口）：

```bash
python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1
DRIVEBRIDGE_FP_SCALE=quick python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1
```

回滚演练：`upload_batches/upload_batch_items/upload_sessions` 为纯新增表，`Operation.batch_id/item_key` 为可空加法列；旧客户端无批次字段的请求指纹不变。`TestFPUploadRecoverySchemaAdditive` 已演练"仅删除新增表后旧数据与旧合同不受影响"。100GiB 真实流与 4 并发显式采样未在本机验证（见 5.2 证据 artifacts/scale-metrics.md）；真实后端与发布独立验收。

## 规格入口

- [批次与恢复](../../../openspec/changes/drivebridge-upload-batch-recovery-v2/design.md)
- [云连接器](../../../openspec/changes/drivebridge-cloud-connectors-v2/design.md)
- [跨存储计划](../../../openspec/changes/drivebridge-cross-storage-transfer-v1/design.md)
