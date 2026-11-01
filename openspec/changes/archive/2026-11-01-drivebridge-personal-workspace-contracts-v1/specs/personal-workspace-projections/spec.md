## ADDED Requirements

### Requirement: 授权后聚合
服务端 MUST 以当前主体可见范围生成首页、计数和分页投影，聚合读取不触发扫描或内容处理。

#### Scenario: 聚合部分不可用
- **WHEN** 容量依赖失效但最近文件可读
- **THEN** 返回各段独立状态与 observed_at，可用段保留，未知值不替换成零。

#### Scenario: 跨租户计数
- **WHEN** 其他用户/租户存在更多文件或任务
- **THEN** 响应条目、总数、游标与错误不泄漏不可见数据。

### Requirement: 原任务身份与状态
服务端 MUST 将任务中心作为既有 Operation/Plan 等 owner 的投影，保持原 ID 与原状态。

#### Scenario: 原任务结果未知
- **WHEN** 底层调用未返回确定结果
- **THEN** ui_state 为 unknown 并保留 owner_ref，不能新建替代任务或宣布失败重做。

#### Scenario: 部分任务恢复
- **WHEN** 客户端请求恢复已部分成功的计划
- **THEN** 经原 owner 执行未完成项，保留成功回执；补偿产生新计划不覆盖原事实。

### Requirement: 调用记录与游标
服务端 MUST 从已认证事实生成调用视图，使用服务端筛选、稳定排序和范围绑定游标。

#### Scenario: 伪造 caller
- **WHEN** 调用者把 caller/task_ref 写成另一个 Agent
- **THEN** 真实 agent_id 不变，source_claim 不能影响鉴权或调用归属。

#### Scenario: 过滤或授权变化
- **WHEN** 使用旧 cursor 查询新筛选或已缩小授权
- **THEN** 拒绝或 cursor_expired，客户端重新查询，不混合不同范围的结果。

### Requirement: 接口生成与兼容
服务端 MUST 以Go DTO/路由登记为字段真源，使用invopop/jsonschema生成JSON Schema2020-12和OpenAPI3.1、openapi-typescript生成TS、类型化Go样例生成fixture，并以开发期Ajv校验，保留旧envelope/工具/错误合同。

#### Scenario: 前后端独立消费
- **WHEN** 前端生成类型和 mock
- **THEN** 从 Go DTO 唯一真源经 B01.2 同一入口生成机器 schema、TypeScript 类型和契约 fixture；HTTP/SDK/MCP 校验同源响应，不能分别维护字段或凭空模拟 supported。

#### Scenario: 生成合同漂移
- **WHEN** DTO 修改未重生成、生成物被手改或 nullable/枚举/fixture 与服务响应不同
- **THEN** B01.3 确定性重生成比较失败，必须从 DTO 修复；expiry_mode 必返、expires_at 必返可 null，旧 agents 响应兼容仍通过。

#### Scenario: PATCH字段三态
- **WHEN** 新请求省略字段、显式传null或传false/零/实际值
- **THEN** DTO解码和生成schema/TS保留presence区别，按每个字段合同更新或拒绝；不能把未提交当清空，fixture覆盖required+nullable与never/duration互斥。

#### Scenario: 未实现能力
- **WHEN** 客户端查询新动作而当前环境未实现/未配置
- **THEN** 区分 unsupported、dependency unavailable 与 forbidden，不把设计或源码清单当 live readiness。
