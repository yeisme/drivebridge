## ADDED Requirements

### Requirement: Configured deployment surface
服务端 CLI SHALL 支持可选部署配置文件与命令树：发现序为显式路径、环境变量、工作目录、用户配置目录、系统配置目录；解析优先级为显式旗标、环境变量、文件、内置默认；未知配置键与不合形状的值 SHALL 被拒绝而不是静默忽略。

#### Scenario: Deployment from config file
- **WHEN** 部署者以配置文件提供 state/listen/origin/issuer/secret 文件路径等部署参数并以 `serve --enabled` 启动
- **THEN** 服务按配置解析并启动，等效于等价的全旗标命令行

#### Scenario: Write switches and secrets stay explicit
- **WHEN** 解析任何配置来源
- **THEN** 默认关闭的入口开关（enabled/development）与凭据值不可来自文件或环境；文件只承载路径、URL 与限额

#### Scenario: Invalid config fails loudly
- **WHEN** 配置文件含白名单外键、带路径的 origin 或非正整数 max_bytes
- **THEN** 命令以错误退出并指出键名，不静默忽略或回退默认
