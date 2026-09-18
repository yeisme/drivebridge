## Why

消费方（首个：DSH `dsh-drivebridge-project-connections-v1`）需要按名字引用多条 DriveBridge 连接（connectionId/label/baseURL/instanceId/spaceId/projectRef/凭据来源类型），但当前 CLI 只有单一全局 `remote_url`/`remote_token_file` 与扁平 8 键配置，Space 表虽有 `Project` 却无连接身份、label 与凭据来源引用。没有 owner-authored 的连接注册表，消费方只能各自注入一次性绑定，无法跨会话/跨项目管理。

## What Changes

新增 Connection 实体与 `drivebridge connection list/add/set/remove/show` 命令组（`--json` 信封）：连接为命名记录，字段含 `connection_id`、`label`、`base_url`、`instance_id`、`space_id`、`project_ref`、`credential_source`（枚举 `local-config`/`env`/`token-file`；`keychain` 仅声明类型、暂无实现）。连接记录持久化在 state 目录 GORM/SQLite，写入只经 CLI；per-connection 的 remote URL 与 token 文件引用下沉到连接记录，保留全局单值作为默认连接的兼容投影。`connection list/show --json` 是消费方的只读投影合同，不输出 token、临时 URL 或绝对路径以外的敏感内容（token 文件路径本身不输出，只输出来源类型与 configured 状态）。

## Capabilities

### New Capabilities
- `drivebridge-connection-registry`: 命名连接注册表与只读投影合同。

## Impact

新增 GORM 表与 CLI 命令组；既有 `space`、`config`、`remote` 行为不变，全局 `remote_url`/`remote_token_file` 继续生效并映射为默认连接。新入口默认关闭（无连接注册表时消费方按现状工作）。双向链接：agent/harness-plugins `openspec/changes/dsh-drivebridge-project-connections-v1`。
