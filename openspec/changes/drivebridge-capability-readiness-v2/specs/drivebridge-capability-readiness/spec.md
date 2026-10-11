## ADDED Requirements

### Requirement: 动作就绪与观测分离
系统 MUST 保留实现、配置、权限、依赖和连接观测的独立事实；未观测不得声明 available。

#### Scenario: 已配置连接但未探测
- **WHEN** 读取空间能力
- **THEN** 返回 connection_observed_at=null 与 unknown，保留恢复入口。

#### Scenario: 失去目录权限
- **WHEN** 再次预检相同 ref
- **THEN** 返回不可执行且不泄露目录内条目。

### Requirement: 无 CLI 客户端恢复
系统 MUST 让纯 MCP 客户端通过 tools/list、inputSchema 和 capabilities 取得参数与恢复指引。

#### Scenario: 客户端未安装 CLI
- **WHEN** 读取上传动作能力
- **THEN** 给出 HTTP 字节路径与原操作回读规则，不要求远端读取本地路径。

#### Scenario: 缺失 mount adapter
- **WHEN** 查询挂载动作
- **THEN** 说明 owner 主机依赖；Agent 无启动挂载权限。

### Requirement: 旧合同兼容
系统 MUST 保留 spec_version、tools、part_size、recovery 及既有键的类型和语义，仅加法扩展。

#### Scenario: 旧客户端解析
- **WHEN** 调用无新参数的 capabilities
- **THEN** 旧响应字段继续可用。

#### Scenario: 未知新 action
- **WHEN** 执行只读 check
- **THEN** 返回 unsupported_action，不猜近似动作。

### Requirement: 兼容、权限与证据分层
系统 MUST 保留既有公开合同和稳定身份；新增能力仅在当前授权内执行，规格校验不得替代运行时与真实后端证据。

#### Scenario: 旧客户端与未启用能力
- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务。

#### Scenario: 仅规格检查通过
- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收。
