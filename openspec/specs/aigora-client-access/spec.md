# aigora-client-access Specification

## Purpose
TBD - created by archiving change drivebridge-aigora-client-access-v1. Update Purpose after archive.
## Requirements
### Requirement: Explicit opt-in key exchange
适配器 SHALL 仅在显式配置 `aigora_url` 时启用 SDK 短期凭证交换；未配置时保持原 domain credential 模式，行为完全一致。

#### Scenario: Legacy mode unchanged without aigora_url
- **WHEN** 未配置 `aigora_url`
- **THEN** 适配器使用既有 domain credential 直连文件服务，不发起任何交换请求

#### Scenario: Aigora exchange when explicitly configured
- **WHEN** 配置了 `aigora_url`
- **THEN** 适配器经 `aigora_key_exchange` 模式用短期凭证访问文件服务

### Requirement: Key isolation and failure semantics
长期 Key SHALL 只发往 Aigora origin，不跟随重定向；交换失败 MUST NOT 静默回退 legacy/BYOK。

#### Scenario: Key never reaches the file service
- **WHEN** 适配器在 aigora 模式下访问文件服务
- **THEN** 文件服务请求体与 Header 不含长期 Key，只携带短期 audience-bound 凭证

#### Scenario: Exchange failure does not fall back
- **WHEN** Aigora 交换失败或返回重定向
- **THEN** 适配器返回原错误并停止，不用长期 Key 直连文件服务、不跟随重定向

### Requirement: Admission stop and honest recovery
服务端返回 `service_admission_required` 时 CLI/MCP SHALL 以稳定错误码停止；恢复 SHALL 使用原 operation ID 与 idempotency key，不自动创建新的付费任务。

#### Scenario: Admission required stops the operation
- **WHEN** 服务端对写路径返回 `service_admission_required`
- **THEN** CLI/MCP 以该错误码呈现并停止，输出不含替换凭据或另走直连的建议

#### Scenario: Resume reconciles with original identifiers
- **WHEN** 操作结果未知需要恢复
- **THEN** 客户端用原 operation ID 查 operation_get 并以原 idempotency key 续传，不产生第二个远程操作

### Requirement: Offline service access contract
`drivebridge://service-access` 资源 SHALL 无需网络可读，说明当前认证模式、工具 schema 发现、适配器文件根范围、HTTP 传输责任、授权与消费确认、未知结果对账方式；已连接客户端不需要安装产品 CLI。

#### Scenario: Connected client without installed CLI
- **WHEN** MCP 客户端读取 `drivebridge://service-access`
- **THEN** 获得认证模式、参数发现、授权边界与恢复说明，且文本不含 token、临时 URL 或本地凭据路径明文

#### Scenario: Tool listing is not authority
- **WHEN** 客户端列出工具
- **THEN** 资源文本明确工具列出不代表当前身份有写权限

