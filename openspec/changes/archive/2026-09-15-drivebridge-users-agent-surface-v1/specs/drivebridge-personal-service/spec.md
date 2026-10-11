## MODIFIED Requirements

### Requirement: Admin-key user provisioning over HTTP
持有 admin 用户密钥的调用方 SHALL 可经 HTTP 批量创建用户，校验与操作员 CLI 一致；普通用户密钥、Agent 令牌与会话 SHALL 被拒绝且不泄露存在性。同一管理面 SHALL 经 MCP 工具与 SDK 提供：批量创建、列出（仅生命周期元数据、永不包含哈希）与按 id 吊销；明文密钥只在签发响应出现一次。

#### Scenario: Admin key creates batch
- **WHEN** admin 用户密钥经用户创建接口提交批量条目
- **THEN** 与 CLI 同规则创建用户并一次性返回明文密钥

#### Scenario: Non-admin rejected
- **WHEN** 普通用户密钥、Agent 令牌或浏览器会话调用用户创建接口
- **THEN** 返回 forbidden，不创建任何用户

#### Scenario: Lifecycle through MCP tools
- **WHEN** admin 用户密钥经 MCP 工具列出并吊销一个用户密钥
- **THEN** 返回不含哈希的生命周期元数据，吊销持久生效，未知 id 返回 not_found；普通密钥调用任一 MCP 管理工具均 forbidden
