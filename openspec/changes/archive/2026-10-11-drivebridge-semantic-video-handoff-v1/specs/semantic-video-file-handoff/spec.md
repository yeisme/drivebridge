## ADDED Requirements

### Requirement: 固定来源的领域交接

DriveBridge SHALL 通过公开合同交付 file ref/version/digest、transfer identity、permission/source facts 与 handoff receipt，保留领域权限和来源版本，且 SHALL NOT 复制其他 owner 私有状态。

#### Scenario: 接受有效的固定来源
- **WHEN** 调用方提供当前有权访问且版本匹配的来源
- **THEN** 服务返回绑定来源的结果与稳定回执

#### Scenario: 来源变化或证据不足
- **WHEN** 来源过期、缺失、跨项目或必需证据不可用
- **THEN** 服务拒绝执行或报告待修订，不伪造完成或采用

### Requirement: 可恢复与诚实的执行状态

新增执行 SHALL 复用原任务身份处理未知结果，并区分测试演示、本地验证和正式接受。

#### Scenario: 提交后连接中断
- **WHEN** 受理结果未知
- **THEN** 读取原操作状态，不重复提交新任务

#### Scenario: 能力未接线
- **WHEN** 运行环境没有该能力
- **THEN** 返回明确 unavailable，不将 fixture 作为正式结果


### Requirement: 制作输入的版本与字节核对

DriveBridge SHALL 将固定 file/version、传输 operation 与实际字节摘要分开记录，并在准备和传输/接收阶段检查版本与当前授权。

#### Scenario: ETag 与字节证据不同
- **WHEN** 仅取得远端版本或 metadata，尚无实际字节验证
- **THEN** 只报告已取得事实，不将版本字段解释为 SHA-256 或媒体已验证

#### Scenario: 传输期间变化或撤权
- **WHEN** 固定来源在准备后或分段传输时变化、失效或撤权
- **THEN** 阻断新的受限操作并保留原记录，不以新版本静默替换

### Requirement: 回存与原传输恢复

DriveBridge SHALL 保存已存在产物并沿原 operation 核对恢复，SHALL NOT 代替生成或正式接受，也不能因应答丢失重复创建远端文件。

#### Scenario: 回存应答丢失
- **WHEN** 目标可能已经写入但客户端未收到结果
- **THEN** 核对原操作与目标版本/摘要，不换名重复上传

#### Scenario: 取消晚到或目标已存在
- **WHEN** 取消与远端结果交错，或本地目标已有数据
- **THEN** 保留真实副作用和旧文件，未经显式覆盖授权不替换

#### Scenario: 无 CLI 客户端
- **WHEN** 客户端只有 MCP 与受控数据通道
- **THEN** 通过实际注册 schema/resource 获取参数，不能假定客户端路径在远端可读
