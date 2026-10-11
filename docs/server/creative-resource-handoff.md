# 创作项目文件交接与回存

本页说明 DriveBridge Server 在 Scaena、Eikona、Sonora 素材复用中的职责。2026-09-16 已实现 Eikona 所需的接收绑定、固定版本读取及 HTTP/SDK 恢复合同，已通过本地 fixture/race 验证；真实部署与其他创作消费者仍独立验收。

## 现有能力和缺口

现有服务拥有固定版本文件、范围受限的 Agent 身份、分片上传、内容校验、下载、配额与操作恢复，见 [公共合同](contract-map.md)。原生 MCP 控制上传/查询，文件字节走 HTTP；连接 MCP 不表示可以读取用户电脑文件。

现有 input.bind_reference 已能记录文件版本、consumer、project_ref、purpose 和摘要，现已委派到统一幂等服务；无 receiver binding 的旧请求不自动取得新读取权，其他创作 owner 的领取导入仍未在本轮验证。工具可见或 ready_for_owner_acquisition 不能作为端到端就绪结论。

## 已落地合同与继续完善的边界

接收绑定、幂等领取、固定版本内容及 status/renew/cancel 已在下方实现记录中说明；选定分组回存和每个消费者的完整闭环仍需各自证据。新增文件级完整交付由 [fixed-delivery-v2](../../openspec/changes/drivebridge-fixed-delivery-v2/design.md) 承接，内容检索交接由 [content-retrieval-v1](../../openspec/changes/drivebridge-content-retrieval-v1/design.md) 承接；两者均为待实施设计。

- 以认证身份核对接收实例和项目，按项目绑定来源目录与回存目的目录；调用方标签或持有引用本身不授予权限。
- 对固定版本进行幂等绑定及受控领取，读取时重新检查权限、文件状态和范围；普通 MCP 输出只包含安全状态与引用。
- 向接收 owner 提供受限字节读取，由接收 owner 完成原媒体校验并保存副本。Server 不保存 Eikona/Scaena/Sonora 领域资产状态。
- 接受来源项目已选定交付版本的文件回存，返回文件版本和校验回执；来源项目负责触发和分组业务状态。多文件与最小来源清单全部成功后才允许报告完整回存。
- 失败保持原操作可查；仅补传缺项。回存不得调用生成 Provider、重新审核或购买容量。

新合同复用现有 SDK、上传与下载 service；本地[实施设计](../../openspec/changes/archive/2026-09-17-drivebridge-creative-handoff-v1/design.md)和[任务](../../openspec/changes/archive/2026-09-17-drivebridge-creative-handoff-v1/tasks.md)已建立，定义了增量 HTTP/SDK 操作和验收入口。Eikona 对应源码合同已实现，新端点尚不能宣称已发布或部署。

## 权限、内容与恢复边界

源读取与目的写入分别授权，旧身份不自动扩权。文件移动后按当前目录检查范围；收到伪造项目名、错误 consumer 或过期身份时拒绝。部署已有多租户治理继续生效，不引入默认跨租户共享。

撤销访问阻止后续授权读取，不保证收回目标项目已导入的副本；既有外部签名 URL 可能在到期前仍可用。临时能力不写进来源清单、普通响应、审计正文或日志。Server 的内容版本也不是永久保存承诺，被清理的版本不能继续声称可读。

来源清单携带来源项目/版本、文件分组、角色、大小和摘要、文件版本关联及安全证据引用。来源项目负责解释角色与审核状态，其他项目再次消费时重新做自己的准入；不能把清单当成授权凭证。

正常交付与回存分别呈现。部分成功可以保留已经写入的文件，但完整标志必须等待全部文件和清单验证。取消传输不删除来源项目成品，关闭新入口不删除既有文件与回执。

## 客户端和验收

未安装产品 CLI 的客户端先通过 tools/list、inputSchema 和可读资源发现当前部署能力。服务可达时由目标项目领取字节；只有电脑能读来源时使用 [本机适配器设计](../creative-resource-relay.md)，不把客户端绝对路径当作 Server 可读路径。

本项目运行证据归 temp/integration-test-runs/<run-id>/，失败也保留。需验证固定版本读取、跨身份/项目拒绝、幂等领取、撤销、分片恢复与逐项回存；实施设计已绑定现有 runner，新增测试仍待实现。既有测试通过不能代替新增消费者闭环测试。

跨项目合同与完整场景见根工作区 [设计](../../../../openspec/changes/archive/2026-09-17-drivebridge-creative-resource-reuse-v1/design.md)、[规格](../../../../openspec/changes/archive/2026-09-17-drivebridge-creative-resource-reuse-v1/specs/creative-resource-reuse/spec.md) 和 [owner handoff](../../../../openspec/changes/archive/2026-09-17-drivebridge-creative-resource-reuse-v1/handoffs.md)。这些工作区链接只提供跨项目背景，独立 checkout 的实施与 closeout 仍由本 owner 的本地 OpenSpec 承担。

## 接收绑定配置与证据

owner 在装有 Server CLI 的终端执行，使用现有 scoped Agent，不签发或扩大该 Agent 权限：

```bash
drivebridge-server handoff-binding create --url https://files.example.com \
  --token-file /absolute/private/owner.key --agent-id RECEIVER_AGENT_ID \
  --space SOURCE_SPACE --prefix references --consumer eikona --project project-a --purpose reference
drivebridge-server handoff-binding list --url https://files.example.com --token-file /absolute/private/owner.key
```

配置通过 `/api/v1/handoff-bindings` 的 owner-authenticated API 或上述 CLI 写入。绑定后 `/api/v1/handoffs` 提供 bind/get/content/renew/cancel；grant ID 本身不是凭据，读取仅允许已登记的接收 Agent。禁用绑定、撤销权限及目录变化会阻止后续读取。

purpose 白名单加法（与本机 `drivebridge bind` 对齐，并保留既有 `reference`）：`reference`、`vault`、`notes-attachments`、`materials`、`saveback`、`owner-store`。未知 purpose 返回 `invalid_binding`。`--consumer` 只是存储标签，不能代替 BindingID 与已登记 AgentID。`GetHandoff` 与 `OpenHandoff` 都按文件**当前** path/space 复核绑定前缀，移出前缀后为 `forbidden`。disable 之后新的 BindReference 与 OpenHandoff 失败，但不删除源文件，也不声称收回消费者已导入的副本。同一 owner+agent+space+prefix+consumer+project+purpose 且未 disable 的创建请求复用原绑定，客户端不必新增幂等键。

本轮 Server/service 与 SDK 全包 race 通过；CLI 构建错误修复后的独立检查也通过。证据分别为 `temp/integration-test-runs/20260916T124045Z-df8a7735/`（service/SDK passed，初次 CLI build failed）及 `temp/integration-test-runs/20260916T124455Z-78fcae0e/`（CLI passed）。保留初次失败事实。

最终 service/SDK/CLI race 证据：`temp/integration-test-runs/20260916T134131Z-cbf92762/`。后续固定旧版本与目录移动回归：`temp/integration-test-runs/20260916T135520Z-e7782a60/`。当前源码通过 vet、CGO_ENABLED=0 构建和本 change 严格校验。真实服务启用与正式发布尚未执行。

## CLI / MCP 排障入口

```bash
drivebridge-server handoff status GRANT_ID --url https://files.example.com --token-file /absolute/private/receiver.key --json
drivebridge-server handoff renew GRANT_ID --url https://files.example.com --token-file /absolute/private/receiver.key --json
drivebridge-server handoff cancel GRANT_ID --url https://files.example.com --token-file /absolute/private/receiver.key --json
```

renew 需要原接收 Agent，不能使用普通发起人身份代替。日常 Eikona 导入由其 adapter 管理 grant；直接续期前需先核实原输入尚未完成、取消或过期。

MCP 无需安装任何 CLI。读取 tools/list 后，可在原 input.bind_reference 参数上额外传 binding_id、consumer_request_ref、RFC3339 request_expires_at；旧必填项与旧返回字段不删除。新动作 input.handoff_status、input.handoff_renew、input.handoff_cancel 只接收原 grant_id（以及原有 tenant/space context）；返回状态、期限与安全元数据。内容仍由已认证的 owner adapter 经 HTTP 获取，不进入 MCP 文本。续期不会改变原请求期限或扩大绑定。

绑定管理仍是用户权限下的 handoff-binding create/list/disable；Agent MCP 不新增配置绑定权限。CLI create 对 agent-id、space、project 做必要选项检查。
