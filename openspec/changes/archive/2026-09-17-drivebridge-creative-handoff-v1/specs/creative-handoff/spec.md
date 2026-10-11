## ADDED Requirements

### Requirement: 接收身份与目录绑定
系统 SHALL 将 consumer/project/purpose 绑定到受信 scoped Agent 身份与源目录；每次读取 SHALL 应用当前 tenant、space、目录和 share 规则。

#### Scenario: 伪造标签或目录迁移
- **WHEN** 调用者伪造项目/consumer，或文件移出授权前缀
- **THEN** 绑定或内容读取被拒绝，不返回跨范围字节

### Requirement: 幂等绑定与可恢复读取
系统 SHALL 以主体和逻辑键唯一约束绑定，相同指纹复用原 grant，不同指纹拒绝；读取 SHALL 不因完成响应丢失而无法恢复。

#### Scenario: 并发重放
- **WHEN** 两个请求以同一键并发绑定同一固定版本
- **THEN** 只有一个 grant；更改摘要或项目后重用该键返回冲突

#### Scenario: 续取与取消
- **WHEN** 短期能力过期而原请求仍有效，或请求已经取消
- **THEN** 前者复核当前权限后续取同一身份；后者不得复活；读取次数不自动证明导入完成

### Requirement: 固定版本的受控传输
系统 SHALL 通过认证内容流交付固定版本，验证大小和摘要；SHALL NOT 将临时 URL、凭据或字节正文放入普通 MCP 输出。

#### Scenario: 内容替换与撤销
- **WHEN** 固定内容已不存在或权限已撤销
- **THEN** 拒绝读取，不降级到 latest，不声称收回已经导入的消费方副本

### Requirement: 回存文件回执不代表领域完成
系统 SHALL 复用现有上传恢复和配额，返回逐项固定版本回执；SHALL NOT 执行生成或判断领域选定状态。

#### Scenario: 文件成功但清单失败
- **WHEN** 文件项完成而来源清单上传失败
- **THEN** 保留成功项及原操作，消费者可补传清单；Server 不报告整个交付分组完成

### Requirement: 在途 creative 与旧合同隔离
系统 SHALL 保留旧 URI、MCP 响应字段、creative 提交和回存状态语义，新交接不得通过生成提交路径完成。

#### Scenario: 新能力关闭
- **WHEN** 关闭新交接或新绑定不存在
- **THEN** 能力发现报告不可用；旧合同回归通过且已保存文件不被删除
