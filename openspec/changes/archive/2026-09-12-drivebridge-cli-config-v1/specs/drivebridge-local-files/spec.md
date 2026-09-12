## ADDED Requirements

### Requirement: Configured CLI surface
CLI SHALL 支持可选配置文件与命令树：发现序为显式路径、环境变量、工作目录、用户配置目录；解析优先级为显式旗标、环境变量、文件、内置默认；未知配置键 SHALL 被拒绝而不是静默忽略。

#### Scenario: Discovery and precedence
- **WHEN** 存在配置文件且同一键同时有环境变量与显式旗标
- **THEN** 显式旗标胜出，其次环境变量，其次文件，且 `config show` 如实报告每键来源

#### Scenario: Unknown configuration key
- **WHEN** 配置文件包含白名单之外的键
- **THEN** 命令以错误退出并列出未知键名，不静默忽略或回退默认

#### Scenario: Security switches stay explicit
- **WHEN** 解析任何配置来源
- **THEN** 默认关闭的写入口开关（enabled/handoff）与凭据值不可来自文件或环境，只能显式命令行；配置面只承载路径与 URL
