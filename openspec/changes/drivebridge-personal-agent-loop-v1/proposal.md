## Why

个人使用需要从文件输入到结果归档的完整闭环。现有能力分散在本机、Server 与 Web，用户需要手工搬运、查目录 ID 或判断版本。本次按已确认的本机优先路线复用既有空间、版本、传输与回执。

## What Changes

本机首用与 Agent 归档：增量来源标签、目录或版本发现、恢复与检索入口。保留旧命令、引用、状态与权限，不新增生成任务或计费平台。具体实现与边界见本 owner README 和 docs。

## Capabilities

### New Capabilities
- drivebridge-personal-agent-loop：本 owner 的个人 Agent 文件闭环与本地验收。

### Modified Capabilities
无已发布合同删除或重定义。

## Impact

所有新字段可选，旧请求和数据可继续使用。CLI/MCP/API 不输出秘密或 Provider 临时链接。只完成本地实现和验证，不包含真实账号、发布、部署或市场招募执行。
