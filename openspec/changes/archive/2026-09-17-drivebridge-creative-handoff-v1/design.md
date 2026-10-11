# 可信创作文件交接

## 基线与边界

internal/service/mcp.go 的 input.bind_reference 目前创建 HandoffGrant；模型已有 Consumer、ProjectRef、Purpose、Digest、Expires、Consumed。HTTP 文件读取和 sdk.Client.Download 已支持固定版本，sdk.Client.Upload 已支持操作恢复。internal/service/creative.go 的用户到创作服务连接是另一条方向的身份，不能将其中保存的 owner token 用作接收方反向认证。

本项目负责文件权限和交接，来源项目负责交付版本及分组完整性。新增能力默认关闭，不借 tenant governance 激活自动扩权；遵守当前 Principal.dataOwner、tenant/space context、目录范围和 governance share 检查。

```mermaid
flowchart LR
  C[已绑定接收项目] --> B[绑定 service]
  B --> G[固定版本 HandoffGrant]
  G --> R[每次授权的内容读取]
  R --> I[消费方原输入会话]
  O[来源项目选定文件] --> U[现有上传与完整校验]
  U --> V[文件版本回执]
```

## 接口与状态决策

新增 typed SDK 操作 BindReference、GetHandoff、OpenHandoff、RenewHandoff、CancelHandoff；均经同一 service，不 shell out CLI。已实现 HTTP 为 POST /api/v1/handoffs、GET /api/v1/handoffs/{id}、GET /api/v1/handoffs/{id}/content、POST /api/v1/handoffs/{id}/renew、POST /api/v1/handoffs/{id}/cancel。这些是实施目标，当前不可作为已发布端点调用。

BindReference 输入沿用 ref、consumer、project_ref、purpose、expected_sha256；HTTP 幂等键来自 Idempotency-Key，旧 MCP 参数转换到同一键。新增绑定记录将这些标签映射至受信 scoped Agent ID、源 space/prefix、可用用途；目的上传使用独立 write scope。用户级配置/应用服务创建绑定，Agent 不能自行增加绑定或签发权限。

绑定请求的发起人和读取者分别记录：有源读取及所需 share 权限的主体可为已登记目标创建 grant，但读取只允许记录中的目标 scoped Agent，不能让发起人的普通权限自动变成目标的长期权限。能力发现增量返回已实现的操作、当前主体是否可执行及阻塞原因；消费方逐项验证，不只匹配工具名。

唯一键包含 data owner、认证主体与逻辑幂等键；请求指纹包含固定版本、接收方、项目、用途和摘要。相同键/指纹返回原 grant，不延长有效期；指纹不同返回 idempotency_conflict。配置绑定变化须重新授权，不能修改旧 grant 的接收方。GORM 事务和唯一约束处理并发，不依赖单进程锁保证幂等。

GetHandoff 返回安全 ID、固定 ref、媒体元数据、摘要、接收方、项目、用途、状态与期限。OpenHandoff 通过同一已认证连接流式读取，只用于精确接收项目；grant ID 不是 bearer 权限。每次重新检查身份、scope、share 规则、文件版本和当前目录，关闭重定向。v1 不返回第三方签名 URL，不要求解密空间的 Provider 直链。

grant 使用原 15 分钟短期窗口。新接入在绑定时附加 consumer_request_ref 与 request_expires_at，固定到接收方持久导入意图及期限；这些是新增可选字段，不增加旧 MCP 调用必填项。无请求关联的旧 grant 保留原单次期限，不开放新续期能力。RenewHandoff 只有原接收方、绑定、文件和用途仍有效且未超过固定请求期限时才可续取，且不得延长请求期限。接收 owner 必须在续取前检查原请求非终态，在取消时调用 CancelHandoff；跨服务通知失败保持待恢复事实，不宣称即时撤销。续取不更换 grant 身份或重建导入。取消幂等，终态不可续取。Consumed 不能被一次读取自动置为不可重试，否则完成响应丢失无法恢复；保留旧字段，新增必要的生命周期字段，不改旧字段含义。

绑定配置与凭据通过现有 CLI/application service 扩展写入，配置只引用用户私有 credential store。第一项实施任务同时添加可发现的配置、inspect、disable 入口及 help，测试中验证实际命令，不要求用户手写 JSON/YAML。

## 回存与在途功能兼容

来源项目冻结交付版本及文件项，以不同于生成提交的键调用现有 Upload。上传键绑定源 owner/project/version/item、目标 space/path 与摘要；重复项复用原 operation/file version，换目标或内容必须换逻辑操作。Server 不解释选定审核、不启动生成、不替来源项目建立业务 outbox。

最小来源清单也是受控上传对象，内容由来源 owner 生成。返回安全 file/version/digest/operation 供来源 owner 汇总；文件全部写入仍不代表清单已完成。复用现有配额、取消和不可默认覆盖规则。不得以 CreativeReceipt.Status=archived 推断新回存分组完整，也不迁移该在途记录。

## 验证命令与任务验收

在本项目运行，以下入口已经存在；新增测试必须接入它们，结果见 docs/creative-resource-handoff.md：

```bash
python3 scripts/check.py -- go test ./internal/service ./sdk -count=1
python3 scripts/check.py -- go test -race ./internal/service ./sdk -count=1
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
openspec validate drivebridge-creative-handoff-v1 --strict --no-interactive
```

tasks.md 的 1.1–1.4 每项均需增加相应失败用例后运行首条；1.5 运行 race 与证据检查；1.6 执行最终命令并同步 docs/creative-resource-handoff.md。检查测试输出确实包含新增用例，不能接受 no tests to run。证据归 temp/integration-test-runs/<run-id>/，六件套由已有 runner 创建，失败保持原退出码并脱敏。

## 迁移与回滚

只加 nullable/default 字段或新表，GORM 管理迁移，无原表重命名或数据删除。关闭新能力保留 grant、上传文件和恢复记录，旧 input.bind_reference 的安全返回形状及其他 creative 路由不删除。新功能测试用 fake consumer 即可完成本地验收；真实三 owner 组合和部署由根验收另报。
