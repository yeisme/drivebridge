## ADDED Requirements

### Requirement: Owner 保持判断与动作隔离

drivebridge SHALL 遵守以下边界：workspace suggestion producer；保持 human-only decision 和受管文件保护。未启用配置 MUST 保持原流程。

#### Scenario: 默认关闭

- **WHEN** 没有显式启用本 owner 判断集成
- **THEN** 不执行 Provider、不读取判断凭据、不生成伪造 live 证据

### Requirement: 修订和原操作身份可恢复

本 owner SHALL 消费公开合同并保留 scope、配置修订与原 request/attempt；未知结果 MUST NOT 被自动重提或静默 fallback。

#### Scenario: 重读未知操作

- **WHEN** 原操作结果未确认
- **THEN** 保留原身份与 unknown 状态，等待核对原回执
