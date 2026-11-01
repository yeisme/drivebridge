## ADDED Requirements

### Requirement: 受限产物上传
服务端 MUST 将 inbox.submit 写入服务端控制的 staging，复用原上传 operation 和固定版本。

#### Scenario: 任意目标回存
- **WHEN** Agent 请求覆盖正式目录中的已有文件
- **THEN** 拒绝；仅允许本 grant 的 staging 新产物，不暴露宽 write 通道。

#### Scenario: 提交响应丢失
- **WHEN** 重复同 key 提交或查询恢复
- **THEN** 返回原 submission/operation，分片和完成请求复检 grant，不能生成重复文件。

#### Scenario: 新 grant 无 CLI 直连收件
- **WHEN** personal_workspace_v1 身份通过 Streamable HTTP MCP 接入并具有 inbox.submit
- **THEN** capabilities 提供专用 POST /api/v1/inbox-submissions HTTP schema；创建返回 submission_id、operation_id、purpose=inbox_staging、part_size、part_path；只通过同范围 HTTP 分片与原 operation_get/complete_upload 完成收件；不得新增隐式 MCP 写工具或将旧 create_upload 适配为收件。

#### Scenario: 旧上传与新收件不得混用
- **WHEN** 新 grant 调用 create_upload 或传入正式 path/file_id，而旧 write 身份使用既有普通上传
- **THEN** 新 grant 请求拒绝；旧身份维持普通上传合同且不自动创建 submission；新收件响应丢失只通过 inbox-submissions/lookup 原键恢复。

#### Scenario: 仅提交动作的读权限
- **WHEN** 仅有 inbox.submit 的有效 grant 查询自己的收件或操作
- **THEN** 可读自身安全状态和 received_parts，不授予固定版本正文读权；正文另需 files.read；失效凭据不得借恢复入口绕过认证，人类可在管理面回查。

### Requirement: 文件接收与幂等归档
服务端 MUST 将人类文件接收决定与归档操作分开，固定 version 与 revision，拒绝不自动删除。

#### Scenario: Agent 自行验收
- **WHEN** Agent 将产物状态设为 accepted 或调用 decision
- **THEN** 拒绝；只由有权人类记录接收/拒绝，不能等同领域采纳。

#### Scenario: 归档未知或冲突
- **WHEN** 同一个归档丢响应或目标已存在
- **THEN** 查询原 operation，冲突要求显式选择，不覆盖且不产生第二次归档。

### Requirement: 文档乐观写入与属性
服务端 MUST 通过现有文件版本应用服务保存文档与属性，保持固定版本和受保护结构化资产边界。

#### Scenario: 过期正文保存
- **WHEN** base_version_id 已不是当前版本
- **THEN** 返回 version_conflict，旧/新版本不变，客户端草稿可保留比较。

#### Scenario: 字段类型迁移
- **WHEN** 尝试把已有属性改 key/type 或任意编辑受保护 policy
- **THEN** 要求 owner 迁移/CLI 服务，不静默转换、批量写 YAML 或绕过元数据权限。

### Requirement: 收藏资源与可重建链接
服务端 MUST 将收藏、资源描述、检查和文档链接保持为引用/元数据，读时重新授权。

#### Scenario: 资源更新版本
- **WHEN** 检查通过后的资源产生新版本
- **THEN** 旧检查仍指旧版，新版本待检查；保存或检查绝不触发安装执行。

#### Scenario: 目标撤权
- **WHEN** 收藏/反链目标不再可读
- **THEN** 不返回敏感名称/正文或扩大权限，删除收藏不删除文件；投影可重建。

### Requirement: 增量持久化与证据
服务端 MUST 使用 GORM 加法迁移、保留旧身份，并给失败/恢复测试保存脱敏 owner 证据。

#### Scenario: 重复迁移与旧数据
- **WHEN** 对旧 File/Agent/Operation fixture 重复运行升级
- **THEN** 不丢 ID/ref、恢复身份或旧默认；新表/列不在同次升级删除旧表。

#### Scenario: 仅设计交付
- **WHEN** 本 change strict 通过但未运行新业务测试
- **THEN** 任务保持未完成，不宣称持久化、真实云或部署已成功。
