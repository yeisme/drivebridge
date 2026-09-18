## ADDED Requirements

### Requirement: 命名连接注册表
系统 SHALL 提供 `drivebridge connection add/set/remove/list/show` 命令组管理命名连接记录，字段包含 connection_id、label、base_url、instance_id、space_id、project_ref、credential_source；写入只通过 CLI/application service。

#### Scenario: 非法字段拒绝
- **WHEN** base_url 非 http(s)、space_id 不存在、connection_id 非 slug 或重复
- **THEN** 命令拒绝且不产生部分写入

#### Scenario: keychain 未实现
- **WHEN** 用户指定 `--credential-source keychain`
- **THEN** 命令明确报错该来源类型未实现，不静默降级

### Requirement: 只读投影不泄露凭据
`connection list/show --json` SHALL 只输出字段白名单与 credential_configured 布尔，不输出 token、token 文件路径、临时 URL 或原始请求。

#### Scenario: 投影字段审计
- **WHEN** 消费方调用 `connection list --json`
- **THEN** 响应仅含白名单字段，凭据以来源类型枚举与 configured 状态出现

### Requirement: 兼容默认连接
注册表为空时系统 SHALL 保持既有全局 remote 配置行为，并将其投影为一条 synthetic 默认连接，不落库。

#### Scenario: 空注册表
- **WHEN** 从未执行 connection add 且全局 remote_url 已配置
- **THEN** list 投影包含 synthetic: true 的默认连接，既有命令行为不变
