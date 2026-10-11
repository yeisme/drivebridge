# 远端租户与空间上下文

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


远端模式接受可选的 `tenant_context` 与 `space_context`。每次调用使用独立 SDK client，避免并行请求串用上下文；上下文只能选择目标，不能扩大身份权限。既有 Agent token 自动绑定原租户和空间，不能继承签发人的管理员权限。

CLI 的 `remote` 命令在标准输入 JSON 中接受这两个可选字段。MCP 客户端即使没有安装 DriveBridge CLI，也可以通过 `tools/list` 的 `inputSchema` 查看字段，通过 `drivebridge://capabilities` 查看工具与传输合同。

服务端的 `approval_required` 表示需要由人进入服务端 Web 的管理授权中心申请修改。CLI/MCP 不提供 Agent 自批或应急升级。CLI 和适配器的本机文件路径仍受本地 root 限制；它们不是服务端可读取路径，实际字节通过受认证 HTTP 传输。

公共 SDK 增加 `client.Scoped(tenant, space)`，返回独立副本；不改变已有构造器和默认单租户调用。服务端迁移和角色引导命令只在部署服务端执行，不要求 MCP 客户端安装服务端 CLI。

验证使用 `python3 scripts/check.py go test -race ./internal/remote -count=1`，并发测试同时发送不同租户请求并确认原 Adapter 上下文不变。

2026-09-16 验证：全项目 race 测试通过，证据 `temp/integration-test-runs/20260916T111031Z-5674731b/`；定向并发测试通过，证据 `temp/integration-test-runs/20260916T110009Z-acc9c370/`。`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge` 与 OpenSpec 验证通过。没有修改本机文件业务或启用真实远端连接。
