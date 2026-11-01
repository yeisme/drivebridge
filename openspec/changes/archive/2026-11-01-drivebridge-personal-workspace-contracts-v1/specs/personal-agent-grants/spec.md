## ADDED Requirements

### Requirement: 加法授权合同
服务端 MUST 新增个人工作台窄授权资源，同时保留旧 agents 入参、五类 scope、默认期限与响应。

#### Scenario: 旧客户端创建
- **WHEN** 旧客户端省略 days 并调用原 POST agents
- **THEN** 沿用默认 7 天、最长 30 天和旧返回结构，不要求新 grants 字段；旧记录缺失期限不能解释为永久。

#### Scenario: 新客户端创建
- **WHEN** 人类创建 read/suggest/submit 范围 grant
- **THEN** server 固定真实 actor/目录/有效期并返回一次 secret，不接受 body 自报 owner。

### Requirement: 显式长期与限时授权
服务端 MUST 仅为新 restriction profile 提供 `expiry_mode: never|duration`，默认 never，响应包含模式和必返可空 expires_at，不以远期日期模拟长期。

#### Scenario: 默认长期
- **WHEN** 本人创建新 grant 且省略 expiry_mode
- **THEN** 显式保存 never，expires_at 为 null；限制 profile 和人类权限仍逐动作生效，旧 Agent 不迁移。

#### Scenario: 六档限时及服务端计时
- **WHEN** 本人选择 duration 及 3600、86400、604800、2592000、7776000 或 31536000 秒
- **THEN** 期限从服务端签发成功时刻计算，server_now >= expires_at 即过期；其他秒数、缺秒数、never 带秒数或未知 mode 返回 invalid_expiry。

#### Scenario: 期限修改与验证
- **WHEN** active/disabled 且未过期的 grant 通过 expected_revision 修改 never/duration 期限
- **THEN** 成功修改时起算并更新 expiry_changed_at；连接验证、幂等重放、禁用/启用、改名或组织变更不重置计时，过期/撤销不能复活。

### Requirement: 全入口窄权限裁决
服务端 MUST 在 HTTP、SDK、MCP、worker 和下载/上传发布中使用同一个权限交集，不能通过旧 endpoint 绕过 restriction profile。

#### Scenario: 标签建议 HTTP 入口与未知恢复
- **WHEN** 新 grant 从 MCP capabilities 发现 labels.suggest 并提交固定版本建议
- **THEN** 使用 POST /api/v1/files/{id}/label-suggestions 与原 Idempotency-Key；返回 suggestion_id/revision/creation_key/state；同键同摘要重放同记录，不同摘要409；GET仅回读自身建议供恢复，不新增MCP写工具、不附送正文files.read或人类采纳权限。

#### Scenario: 建议授权绕旧写接口
- **WHEN** labels.suggest Agent 调用旧 write 或修改人工标签
- **THEN** 拒绝且保留审计，不用宽 write scope 实现建议权限。

#### Scenario: 跨 Agent staging
- **WHEN** 同用户 Agent B 读取/完成 Agent A 的 staging operation
- **THEN** 拒绝并不泄漏 A 的资源名称或结果；相邻前缀、路径编码变体同样拒绝。

#### Scenario: 多 Agent 对同一文件建议标签
- **WHEN** Agent A 与 B 向同一个固定版本分别提交建议
- **THEN** 保留独立来源和幂等结果，B 不能替换 A 的建议；只有人类采纳可向 manual 添加标签，已有人工标签不丢失。

#### Scenario: 人类采纳与版本竞争
- **WHEN** 两个人类请求同时对同建议提交相同expected_revision，或base_version已经过期
- **THEN** 建议状态和manual标签增量合并在同事务CAS；冲突方不写，保留已有人工标签；只准采纳建议子集，Agent不得自行decision。

#### Scenario: 建议决定响应丢失
- **WHEN** 已提交的采纳/拒绝丢响应后使用相同幂等键查询或重放
- **THEN** 返回原决定与可回读标签事实，不新建建议/决定或重复覆盖；重启不丢来源、版本和结果，S2交付不等待收藏/资源库。

### Requirement: 授权修订与撤权发布
服务端 MUST 以 revision 和有效期控制 grant 变更，并在最终文件发布前重新鉴权。

#### Scenario: 撤销与完成竞争
- **WHEN** grant 在上传后、publish 前被撤销
- **THEN** 未发布的写入失败关闭，已发布的固定版本保留事实，不宣称撤销已撤回字节。

#### Scenario: 并发修改授权
- **WHEN** 两个更新携带同一个 expected_revision
- **THEN** 仅一个成功，另一个返回冲突；扩大权限必须由人类明确请求。

### Requirement: 一次 secret 与幂等恢复
服务端 MUST 只交付一次明文 secret，保留可回查的创建身份，轮转使旧凭据失效。

#### Scenario: 创建丢响应
- **WHEN** 相同 key 的创建再次到达
- **THEN** 返回同 grant metadata 和 not_replayable，不存明文供重放，不新增身份。

#### Scenario: 轮转丢响应与期限保持
- **WHEN** 未过期 grant 的轮转提交后响应丢失
- **THEN** 原键 rotate/lookup 返回原 grant/agent 与凭据代次 metadata，secret 不重放、旧凭据立即失效；expiry_mode/expires_at 不变，只有人类显式使用新键及最新 revision 才能再次轮转。

#### Scenario: 终态重新接入
- **WHEN** 已过期或已撤销 grant 要重新接入
- **THEN** PATCH/rotate 拒绝恢复旧身份，显式创建新 grant 并保留旧调用/审计事实。

#### Scenario: 轮转与降级
- **WHEN** 仍有新 restriction profile 凭据时要求运行不识别它的旧 binary
- **THEN** 禁止不安全降级，必须先执行文档中的停签发/隔离/撤销与验证步骤。

### Requirement: 人类管理与安全观察
服务端 MUST 将授权管理和范围扩展限制给有权人类，观察结果不得冒充远端 Agent 在线。

#### Scenario: Agent 自我扩权
- **WHEN** 普通 Agent 调用 PATCH grant 或组织策略
- **THEN** 拒绝，不因同一 owner 或 caller 声明授予管理权。

#### Scenario: 检查配置通过
- **WHEN** 人类完成 grant 配置检查
- **THEN** 仅返回带时间的 checks，未知远端连接保持 unknown，不启动文件操作，不重置期限，不主动派发外部 Agent 任务。

### Requirement: 个人 Agent 分组与置顶
服务端 MUST 在本人 tenant/user 范围持久化分组 CRUD、归组与置顶，使用独立组织 revision，不影响 grant 或文件权限。

#### Scenario: 跨设备回读
- **WHEN** 本人在设备 A 创建分组并归组、置顶，设备 B 刷新或服务重启后读取
- **THEN** 返回同一 Group/Organization 事实及 revision，支持旧 Agent 而不迁移其授权；其他用户/租户不可见，Agent bearer 不可管理。

#### Scenario: 删除分组
- **WHEN** 删除组与另一设备归组并发
- **THEN** 同事务和 revision/fencing 保证无悬空归属；删除只解绑并递增组织 revision，保留 pinned、Agent/grant 身份、文件授权与期限，不调用 revoke。

#### Scenario: 组织冲突和列表恢复
- **WHEN** 两设备同 revision 改名/归组/置顶或用修改前 cursor 翻页
- **THEN** 写入只一方成功、另一方 revision_conflict；名称冲突 name_conflict；旧游标 cursor_expired 后重查，不靠本机最后写入覆盖服务端。
