## ADDED Requirements

### Requirement: CLI parity across modes
传输与组织命令 SHALL 在本地与远程模式间保持等价：远程路由由解析后的 remote_url 决定，输出合同同一；远程不支持的能力以固定码拒绝而不是静默降级或本地伪造。

#### Scenario: Remote CLI transfer
- **WHEN** 配置解析出 remote_url 且用户执行 upload/ls 等传输命令
- **THEN** 命令经远程 adapter 执行并以同一 operation envelope 报告，幂等键派生规则与本地一致

#### Scenario: Unsupported remote capability
- **WHEN** 命令依赖远程服务端尚未提供的端点
- **THEN** 返回 remote_unsupported 固定码并指明缺口，不在本地伪造索引或状态
