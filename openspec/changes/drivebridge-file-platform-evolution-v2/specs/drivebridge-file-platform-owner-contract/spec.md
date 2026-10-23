## ADDED Requirements

### Requirement: 必需能力与唯一owner
工作区 MUST 为十四项必需能力及跨owner消费建立明确owner/spec/tasks/验收关系，不因分期删除能力或复制canonical状态。

#### Scenario: 拆分Web与服务任务
- **WHEN** 一个能力同时涉及服务端、Client与本机适配
- **THEN** 服务状态归Server、交互归Client、本机范围归本机owner，根只提供交接路径。

#### Scenario: 既有Manju任务已存在
- **WHEN** 设计DriveBridge内容检索联调
- **THEN** 新建联调包引用原任务，不覆盖Inbox/Understanding/Retrieval等旧任务或制造第二个检索owner。

### Requirement: 事务分析与内容分层
系统 MUST 保留事务与授权真源，ClickHouse和DuckDB仅保存可重建分析副本，Manju保持内容理解与语义检索owner。

#### Scenario: 分析后端故障
- **WHEN** ClickHouse或DuckDB不可用
- **THEN** 文件业务不依赖其写入成功，查询明确降级或不可用，不回传虚假零值。

#### Scenario: 查询后执行整理
- **WHEN** Agent将分析候选变为写入计划
- **THEN** 重新固定确切资源和版本并由事务owner校验当前授权，不以索引结果批准修改。

### Requirement: 固定身份和权限交接
系统 MUST 保持instance/file/version/operation身份及双边授权；MCP只传安全引用，字节由受控HTTP或owner适配器传输。

#### Scenario: 创建响应丢失
- **WHEN** 客户端不知道上传是否受理
- **THEN** 读取同键同指纹原请求，不换身份、目标或幂等键创建替代操作。

#### Scenario: 内容producer能力缺失
- **WHEN** 音视频理解接口未就绪或能力已退役
- **THEN** 返回unsupported或missing并保留所需能力的后续任务，不伪造证据、不复活Anatomia独立ASR/OCR。

### Requirement: 证据与规模声明
工作区 MUST 将设计、源码实现、本地验证、真实后端和发布部署分开，性能目标未经相应测试不得声称达到。

#### Scenario: 所有spec通过strict
- **WHEN** 尚未执行实施任务
- **THEN** 只能报告规格完整，不能声称百万文件/100GiB或真实云模型已验收。

#### Scenario: 跨仓已有dirty变更
- **WHEN** 本轮完善spec和文档
- **THEN** 保留无关修改与既有任务状态，仅变更明确归属的设计文件。
