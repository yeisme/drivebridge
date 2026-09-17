# drivebridge-aigora-client-access-v1

## Why

远程文件服务迁移到统一身份后，本机 CLI/MCP 需要把长期 Key 仅发送给 Aigora 交换短期凭证，而不是直接发给文件服务；客户端必须能在不安装产品 CLI 的情况下理解认证模式、授权边界与恢复合同。

## What Changes

- 显式配置 `aigora_url` 时适配器经 `sdk.NewAigora` 做短期凭证交换；未配置时保持原 domain credential 模式，行为不变。
- 长期 Key 只发往 Aigora origin，不跟随重定向，交换失败不静默回退 legacy/BYOK。
- 服务端返回 `service_admission_required` 时 CLI/MCP 以稳定错误码停止，不建议替换凭据或直连；恢复使用原 operation ID 与 idempotency key，不自动创建新付费任务。
- 新增 `drivebridge://service-access` 无需网络可读的 MCP 资源，说明认证模式、参数发现、文件根范围、HTTP 传输责任、授权与消费确认、未知结果对账方式。

## Capabilities

### New Capabilities
- `aigora-client-access`: Aigora 短期凭证交换与客户端访问合同。

### Modified Capabilities

## Impact

仅远端适配器、配置允许键清单与 CLI 错误映射；不改变本机空间模型，不持有权益真值、账本或远程文件 ACL。尚无真实云账号、远程服务或生产部署验收。
