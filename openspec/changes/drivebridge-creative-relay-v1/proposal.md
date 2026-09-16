## Why

远程创作项目不能读取用户电脑路径；现有 DriveBridge 本机文件能力、remote.Adapter 和 Scaena socket 不能自动组成三个项目的引用导入。需要复用现有传输，补齐本机主动中转及原操作恢复。

## What Changes

- 为已绑定目标项目提供受控本机中转，支持 Scaena、Eikona、Sonora 的原输入会话。
- 记录固定来源、目标会话与原传输操作关联；进程退出、网络中断及完成响应丢失后恢复原任务。
- 增加同一 service 的 CLI/MCP 入口和能力发现；不要求安装三个创作项目 CLI。
- 区分同机、服务可达和仅本机可读三种路径；不创建公网 listener 或通用调度 daemon。

## Capabilities

### New Capabilities
- `creative-relay`: 本机素材到创作 owner 输入会话的安全中转与恢复。

### Modified Capabilities
无。旧 upload/download、引用、空间、state 锁和 Scaena handoff 保持原义。

## Impact

Owner 为 mcp/drivebridge；修改范围为 internal/bridge、internal/remote、配置/CLI、MCP registry、相关测试和 docs/creative-resource-relay.md。目标服务使用本地 fake HTTP 验收，不把其他仓库发布作为本地任务。

本包为待实施规格，任务全部未完成。既有 OneDrive 与 generic 后端复用各自字节读取能力，不引入新后端或 Provider 调用。
