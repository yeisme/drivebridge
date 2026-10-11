## Why

DriveBridge 的固定文件版本与分片传输已经存在，但 input.bind_reference 只记录接收方标签，尚无本 change 验证的幂等领取闭环。创作项目需要以可信项目身份导入副本，并将选定交付文件回存；这些行为不能依赖 DriveBridge 代为提交生成任务。

## What Changes

- 将 HandoffGrant 的创建、查询、受控内容读取、续期和取消收敛到 application service，并以 scoped Agent 身份绑定 consumer、项目、用途与目录。
- 为现有 HTTP/SDK 增量提供同一交接合同，旧 MCP input.bind_reference 委派到该 service；补齐其幂等键语义。
- 复用既有 Upload/Download、版本、配额和受控配置入口，为来源项目提供逐项回存及安全回执。
- 用本地 fake consumer、真实数据库和 HTTP 验证身份、版本、重放、撤销及兼容行为；不运行真实网盘或生成 Provider。

## Capabilities

### New Capabilities
- `creative-handoff`: 可信接收项目、幂等交接与范围化文件回存。

### Modified Capabilities
无删除或重定义。现有本机/远程 URI、上传会话及其他 creative 路由保持原义。

## Impact

Owner 为 backend-server/drivebridge，涉及 internal/service、sdk、CLI 配置与本地文档。新增表或可选字段采用 GORM 增量迁移，保持 CGO_ENABLED=0。

在途 creative.go/creative_save.go 有 CreativeConnection、CreativeReceipt、CreativeAsset 和生成提交/轮询回存路径。本 change 不修改其功能范围或生成权限，不把它们的 submitted/archived 状态改为新交付分组语义。可复用经过验证的读取/上传原语，不调用 submitCreative 或 startCreativeSaveback 来实现新交接。

本包只完成实施规格；tasks.md 的运行时任务全部待完成。客户端、三个创作 owner 的实现或部署不作为本项目本地 closeout 任务。
