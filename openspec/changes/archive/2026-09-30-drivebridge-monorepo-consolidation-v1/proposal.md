## Why

用户明确要求合并 DriveBridge 多个子项目以降低开发管理成本。当前 CLI/MCP、Server、Web 是三个子模块并用相对路径交叉构建，日常修改需要三个 Git 和 OpenSpec 上下文。

## What Changes

- 将现有 DriveBridge 仓库迁到 `cli/drivebridge`，Server 和 Web 作为 `server/`、`web/` 普通目录导入。
- 一个 Git、一个 active OpenSpec、一个文档目录和一组开发/构建/验证入口；所有必需产品能力保留。
- 保留两份既有 Go module 名称，CLI 本地 replace 改指 `./server`，不破坏外部 SDK import、命令和 wire 合同。
- 旧目录保留指向新位置的兼容链接；旧 Git 元数据/索引与迁移前文件摘要留在私有本地备份，不删除未提交工作。
- 不包含产品前端重写、生产数据迁移、远端创建/推送/发布。

## Capabilities

### New Capabilities
- `drivebridge-unified-development`：单仓开发、统一规格、兼容和迁移验证。

### Modified Capabilities
无协议或数据库的破坏性变更。源码路径迁移显式记录；旧路径兼容至少一个发布周期，无确定移除版本前继续保留。

## Impact

`cli/drivebridge` 成为唯一维护 owner；前端只消费内部公开 API，仍不拥有文件/权限真源。root 只改子模块/路由/Skill profile 和交接导航。历史仓库不删除，远端仍未更新。
