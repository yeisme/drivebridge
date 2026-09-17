# remote-tenant-context Specification

## Purpose
TBD - created by archiving change drivebridge-tenant-context-v1. Update Purpose after archive.
## Requirements
### Requirement: Context is not authority
远端客户端 SHALL 传递可选租户与空间上下文，保持每次调用独立，由服务端验证访问权限。

#### Scenario: Concurrent calls select different tenants
- **WHEN** 两次调用选择不同租户
- **THEN** 使用独立 SDK client 上下文，不修改共享 Adapter

#### Scenario: Client without an installed CLI
- **WHEN** MCP 客户端查询 tools/list 与能力资源
- **THEN** 获得上下文参数、授权失败恢复入口及本机文件访问边界

