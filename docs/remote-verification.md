# 远程模式验证

原有本机模式回归/race：`temp/integration-test-runs/20260911T071552Z-dcce1787/`。

真实 stdio 远程闭环：`temp/integration-test-runs/20260911T073929Z-962e8635/`。测试启动本机文件服务与 OIDC/S3 fixture，再通过真实 stdio MCP 上传、下载、重放、拒绝路径逃逸、验证撤销。没有输出 Agent token 或临时 S3 URL。

vet 与 pure-Go 构建分别见 `temp/integration-test-runs/20260911T071557Z-33c9e026/`、`temp/integration-test-runs/20260911T071558Z-f7fe533e/`。远程 SDK 当前使用相邻 owner 的本地 replace，尚未单独发布。

## 认证模式与恢复（aigora-client-access）

- 未配置 `aigora_url`：保持原 domain credential 模式，行为不变。配置后适配器经 `aigora_key_exchange` 把长期 Key 仅发往 Aigora origin 交换短期凭证；文件服务只收到 audience-bound 短期凭证。两侧 HTTP client 都不跟随重定向，交换失败不静默回退 legacy/BYOK。
- 本机侧只负责凭证交换与请求签名；权益真值、账本、远程文件 ACL 归远程服务。服务端返回 `service_admission_required` 时 CLI/MCP 以该稳定错误码停止，输出不含替换凭据或直连建议。
- 恢复：用原 operation ID 查 `operation_get`，以原 idempotency key 续传；绝不自动创建第二个付费任务。MCP 客户端无需安装产品 CLI，读取 `drivebridge://service-access` 资源即可离线获得认证模式、参数发现与恢复说明。
- 适配层 fake 服务回归覆盖：Key 隔离、重定向拒绝、交换失败不回退、准入停止、原标识恢复；证据见对应 `temp/integration-test-runs/<run-id>/`。尚无真实云账号、远程服务或生产部署验收。
