## Why

唯一 owner 为 `cli/drivebridge`，CLI/MCP、server、web 是同仓模块。internal/service、sdk 路径相对 server/；OpenSpec、任务工具与统一证据在仓库根，命令位置见 [开发入口](../../../docs/development.md)。

当前原型提供单人多 Agent、目录/动作级授权、任务详情与产物验收。现有 Server 已有文件平台合同，但 Agent 只有 read/write/delete/import/export 粗 scope，缺少工作台投影、窄授权、受限收件与知识资源元数据。直接把原型勾选映射到 write 会造成越权。

## What Changes

- 准入 fit：文件、授权、收件、版本与回执在本 Server；Client、MCP/CLI 和领域内容 owner 通过公开合同消费。
- 新增工作台聚合、任务与调用分页投影；保留原 Operation/Plan 为真源。
- 新增 additive agent-grants，强制 files.read/labels.suggest/inbox.submit 全入口限制；新 profile 默认 never，duration 六档，服务端计时；轮转保身份/期限、旧凭据立即失效，旧 agents 默认 7 天/最多 30 天/响应不变。
- 增加服务端个人 Agent 分组 CRUD、归组/置顶、独立 revision/跨设备回读；删除组只解绑不撤权。
- 新增受限 staging、人工收件决定/幂等归档、收藏/属性/文档新版本/资源检查合同。
- B01固定Go DTO/路由登记→JSON Schema2020-12/OpenAPI3.1→TS，类型化Go样例fixture和Ajv开发校验；生成工具/开发依赖本轮不实现不安装。
- SQLite单机优先、PostgreSQL保留adapter且实际迁移/规模另验；复用net/http/GORM/任务运行时，不新增MQ或微服务。
- 标签建议从B09.1拆至S2 B16，最小收件链与完整Agent验收分别用B12.2/F19和F23；补齐并发、游标、secret未知恢复与安全降级，本轮不执行真实迁移。

## Capabilities

### New Capabilities
- `personal-agent-grants`：窄授权、凭据生命周期、受限执行与撤权。
- `personal-workspace-projections`：聚合、任务、调用与真实能力状态。
- `personal-workspace-assets`：收件、文档/属性、收藏与资源持久化。

### Modified Capabilities
现有 API、SDK/MCP 名称、Agent scopes/defaults、文件/版本/Operation ID 与旧响应结构保留。新增 restriction profile 是加法能力，但不认识它的旧 Server 禁止接管新 grant 的鉴权。

## Impact

内部实现落 internal/service、sdk、fixture 与必要文档/合同生成命令。GORM/pure-Go 与已有运行时不变；不新增网关、通用 Agent runtime、生成任务或文件副本栈。PRD R01–R15 保留；B01 明确交付 Go DTO→schema/TS/fixture 统一生成与漂移门，S2 最小 Agent 联调不等待 S3 信息工作台，S5 的内置助手/原生能力需原 owner 合同后继续。
