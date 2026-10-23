# 设计

## 能力与所有权

DriveBridge 拥有用户、文件版本、对象存储和交接授权；Gateway 拥有 MCP 聚合、调用者身份传递、工具目录投影、审计和生命周期；Scaena、Eikona、Sonora 拥有各自输入校验、任务状态和最终资产。

## 共享实例访问

使用现有独立用户密钥。Gateway 必须按调用者绑定下游凭据，不能把所有请求转成共享管理员身份。普通用户的文件列举、搜索、变更流和统计工具在 `tools/list` 中隐藏；服务端仍对猜测的工具名返回结构化引导错误。完整空间 Agent 和 admin key 保持兼容。

## 稳定引用与绑定

上传回执返回版本化 `drivebridge://<instance>/file/<file-id>/version/<version-id>` 引用、摘要、状态查询入口和恢复动作。新增 `input.bind_reference`，绑定用户、文件版本、consumer、project、purpose、摘要、限制和过期时间。owner 只能兑换自己的有效绑定；绑定失败保留原引用和幂等键。

## 错误信封

所有 owner 和 Gateway 的不可用响应统一包含 `reason`、`impact`、`next_actions`、`retryable` 和 `recovery_tool`。错误动作描述使用真实 MCP 工具名；不得泄漏 URL、token、正文或 provider payload。

## 字节传输

小文件使用受限 `input.upload_base64`；大文件使用 DriveBridge/对象存储暂存对象。MCP 负责控制、授权、校验和完成，字节由受控对象存储传输。owner 流式校验 SHA-256、MIME 和大小后再导入 canonical storage。
