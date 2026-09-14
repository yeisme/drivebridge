# drivebridge-remote-client Specification

## Purpose
TBD - created by archiving change drivebridge-remote-client-v1. Update Purpose after archive.
## Requirements
### Requirement: Personal identity isolation
系统 SHALL 校验用户、Agent 和资源归属，OIDC 登录不自动授予 OneDrive 文件访问。

#### Scenario: Another user requests a file
- **WHEN** 用户请求他人的文件、版本、操作或连接
- **THEN** 返回拒绝且不泄露内容或存在性

#### Scenario: Agent authority revoked
- **WHEN** Agent token 已撤销、过期或超出目录/操作范围
- **THEN** 拒绝后续请求

### Requirement: Durable file transfer
系统 SHALL 保留稳定文件身份、不可变版本和原操作恢复。

#### Scenario: Interrupted upload
- **WHEN** 分片上传中断或服务重启
- **THEN** 查询原 operation 及已接收分片，校验相同源文件后继续，不创建重复文件

#### Scenario: Replayed complete
- **WHEN** 客户端重放已完成操作
- **THEN** 返回同一文件版本与回执

### Requirement: Safe user and Agent interfaces
系统 SHALL 通过统一服务提供网页/API/CLI/MCP，临时凭据不得进入普通 MCP 结果和日志。

#### Scenario: Remote client path
- **WHEN** 远程 MCP 收到客户端文件路径
- **THEN** 不把路径当作远端文件，指引宿主通过 HTTP 传输字节

#### Scenario: User changes file version
- **WHEN** 当前版本与调用者指定版本不同
- **THEN** 不覆盖文件并返回版本冲突

### Requirement: CLI parity across modes
传输与组织命令 SHALL 在本地与远程模式间保持等价：远程路由由解析后的 remote_url 决定，输出合同同一；远程不支持的能力以固定码拒绝而不是静默降级或本地伪造。

#### Scenario: Remote CLI transfer
- **WHEN** 配置解析出 remote_url 且用户执行 upload/ls 等传输命令
- **THEN** 命令经远程 adapter 执行并以同一 operation envelope 报告，幂等键派生规则与本地一致

#### Scenario: Unsupported remote capability
- **WHEN** 命令依赖远程服务端尚未提供的端点
- **THEN** 返回 remote_unsupported 固定码并指明缺口，不在本地伪造索引或状态

