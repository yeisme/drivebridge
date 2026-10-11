# 工作台合同基线（B01.1）

本轮只读核对旧合同，并为后续新增合同建立保护范围。新schema/DTO不代表对应HTTP处理器已注册；新grant不能复用旧宽write来绕过restriction profile。

| 合同 | 源码位置 | 当前事实 | 后续保护/接线 |
| --- | --- | --- | --- |
| Agent身份 | server/internal/service/model.go Agent；auth.go authenticate/createAgent | 旧scope字符串、非空Expires、token hash；当前每次bearer认证按旧期限检查 | 原/agents默认7天及最多30天保持；新never必须走独立profile，不用远期哨兵 |
| 资源授权 | model.go allowOn/file/operation | 事务内复读Agent；检查owner/space/prefix/scopes/期限；transfer映射write或export | 新grant所有HTTP/SDK/MCP/worker入口需同一action裁决；不能仅改authenticate后放行旧write |
| 固定引用 | discovery.go PinnedFile；model.go FileRef | resolveRef指定file/version，回读版本并覆盖返回视图的版本相关字段，不静默升级 | 保留旧引用身份与版本漂移行为，客户端不得用current版本替换 |
| 普通上传 | http.go uploads；mcp.go create_upload/complete_upload；sdk uploadbatch.go | 原operation/分片/恢复和旧幂等键合同已存在；与新submission不同 | 新grant旧create_upload拒绝；专用收件HTTP创建purpose=inbox_staging并复用受限分片/complete |
| HTTP成功与错误 | http.go respond/writeError；error_messages.go | 成功data信封，错误error.code/message；新invalid_expiry已增量登记422 | 不改旧端点错误映射和SDK既有解析；新增错误只补明确登记 |
| SDK旧模型 | server/sdk/uploadbatch.go、organization.go、client.go | 旧分页/可空operation、请求身份和typed error已有公开合同 | 新工作台Page不批量改旧SDK列表与json tags |
| MCP | service/mcp.go与根internal/bridge/mcp.go、internal/remote/remote.go | server直连是Streamable HTTP，根adapter可涉及宿主local_path | 新HTTP动作通过capabilities发现，不虚构写工具；字节通道与控制面分开 |
| 新合同生成 | server/internal/contract、cmd/drivebridge-contracts | DTO、schema、fixture与TS已开始生成；全部新增路由标planned | 仍缺部分DTO和完整端点接线，不因生成成功标B01.2/B01.3完成 |

## 实际保护证据

- 旧HTTP错误/成功信封及脱敏定向测试通过；增量invalid_expiry从400修为422，旧错误用例不变。证据：temp/integration-test-runs/20261001T110833Z-1f42bbb2。
- 本地Server/OIDC/S3 fixture验证旧Agent省略days仍7天、31天拒绝、GET仍数组、创建仍有agent/token。证据：temp/integration-test-runs/20261001T105140Z-1d691b8f。
- 此处未宣称旧上传幂等全量、真实MCP新动作、真实网盘或多实例验证通过；这些保持后续任务验收。

本基线不清理未提交源码，不变更现有真实身份/配置，不运行race。B01.1为合同清点，不是业务实现退出门。
