# handoff-binding-prefix Specification

## Purpose
TBD - created by archiving change drivebridge-server-bind-prefix-v1. Update Purpose after archive.
## Requirements
### Requirement: purpose 白名单加法

系统 SHALL 在 `CreateHandoffBinding` 接受 purpose `reference`、`vault`、`notes-attachments`、`materials`、`saveback`、`owner-store`。未知 purpose SHALL 拒绝并返回 `invalid_binding`。SHALL NOT 删除 `reference`。

#### Scenario: 允许的新 purpose
- **WHEN** 数据所有者创建绑定且 `--purpose` 为 `vault`、`notes-attachments`、`materials`、`saveback` 或 `owner-store`
- **THEN** 绑定成功，返回的 `purpose` 与请求一致

#### Scenario: 保留 reference
- **WHEN** 创建绑定且 purpose 为 `reference` 或省略 CLI 默认值
- **THEN** 绑定成功，purpose 为 `reference`

#### Scenario: 未知 purpose 拒绝
- **WHEN** 创建绑定且 purpose 为白名单之外的值
- **THEN** 返回 `invalid_binding`，不写入绑定

### Requirement: 读取按当前路径复核前缀

`GetHandoff` 与 `OpenHandoff` / `readableHandoff` SHALL 使用文件的**当前** `path` 与 `space_id` 对照绑定 `prefix` 与 `space_id`。文件已移出前缀或空间时 SHALL 返回 `forbidden`，SHALL NOT 返回跨范围字节。越界错误码 SHALL 保持 `forbidden`。

#### Scenario: 移出前缀后内容读取拒绝
- **WHEN** 已绑定前缀 `refs` 的文件被移动到 `outside/source.txt` 后调用 `GetHandoff` 或 `OpenHandoff`
- **THEN** 返回 `forbidden`，不返回文件字节

#### Scenario: 仍在前缀内允许
- **WHEN** 同一文件再移回 `refs/source.txt` 后由已登记 Agent 调用 `OpenHandoff`
- **THEN** 读取不被前缀检查拒绝

### Requirement: disable 停止新读取且不删除副本

`POST /api/v1/handoff-bindings/{id}/disable` 之后，该绑定的 `BindReference` 与 `OpenHandoff` SHALL 失败（`forbidden`）。系统 SHALL NOT 删除源文件，SHALL NOT 声称已收回消费者已导入的副本。

#### Scenario: disable 后 Bind 与 Open 失败
- **WHEN** owner disable 某绑定后，原 Agent 再 `BindReference` 或 `OpenHandoff`
- **THEN** 均返回 `forbidden`

#### Scenario: 源文件仍在
- **WHEN** disable 之后 owner 读取原文件
- **THEN** 文件仍可读；disable 回执不含已删除副本的声明

### Requirement: 调用方标签不是认证

系统 SHALL 拒绝 Agent 调用 `CreateHandoffBinding`（`forbidden`）。授权 SHALL 为 BindingID 加上绑定中登记的 AgentID。请求里的 `consumer` 字符串 SHALL 与绑定一致，但 SHALL NOT 单独构成认证。

#### Scenario: Agent 不能创建绑定
- **WHEN** scoped Agent 调用 `POST /api/v1/handoff-bindings`
- **THEN** 返回 `forbidden`，不创建绑定

#### Scenario: 仅 consumer 字符串不能越权
- **WHEN** 另一 Agent 使用相同 `consumer` 但不是绑定登记的 AgentID 去 `BindReference` 或 `OpenHandoff`
- **THEN** 返回 `forbidden`

### Requirement: 相同作用域绑定可复用

同一 owner、agent_id、space_id、prefix、consumer、project_ref、purpose 且未 disable 的创建请求 SHALL 复用已有绑定。省略幂等键的客户端 SHALL 仍可创建；不同字段产生新绑定。

#### Scenario: 同字段复用
- **WHEN** owner 以相同 agent/space/prefix/consumer/project/purpose 再次创建且原绑定未 disable
- **THEN** 返回同一 binding id，不插入第二行有效绑定

#### Scenario: 不同前缀是新绑定
- **WHEN** 同一 agent 与 consumer 但 prefix 不同
- **THEN** 创建另一绑定，原绑定不变

