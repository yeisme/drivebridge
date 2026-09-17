# Aigora Key 远程客户端接入

本变更只负责本机 CLI/MCP 到远程文件服务的认证交换，不持有权益真值、账本或远程文件 ACL。原有 domain credential 模式保留；只有显式配置 `aigora_url` 才启用 SDK 的短期凭证交换。适配器从既有私有 `remote_token_file` 读取 Key，不将其发往 DriveBridge、不跟随重定向，也不在失败时自动回退到 legacy/BYOK。

`drivebridge://service-access` 是无需网络可读的资源，说明当前认证模式、工具 schema 发现、适配器文件根范围、HTTP 传输责任、授权与消费确认、未知结果对账方式。已连接客户端不需要安装产品 CLI；宿主配置命令不属于客户端必需路径。`local_path` 指适配器宿主允许根内的文件，不假设客户端路径可被远程读取。

工具列出不代表当前身份有写权限。当前后端统一身份写路径尚未完成商业准入，返回 `service_admission_required` 时必须停止，不能替换 Key 或另走直连。恢复时使用原 operation ID 和 idempotency key；不自动创建新的付费任务。

remote、config、CLI 回归与 race 通过，纯 Go 构建和 vet 通过。尚无真实云账号、远程服务或生产部署验收。
