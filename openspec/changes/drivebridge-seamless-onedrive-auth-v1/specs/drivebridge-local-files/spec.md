## ADDED Requirements

### Requirement: Seamless first-use onboarding
auth SHALL 以 device-code 流完成 OneDrive 登录并自动发现默认 drive 与可选 root，不要求用户注册应用或手工查询 Graph；凭据与令牌不得出现在任何输出。

#### Scenario: One-command first login
- **WHEN** 用户在终端运行默认 auth 且在浏览器完成设备码确认
- **THEN** 命令写入 rclone 配置、报告发现的 drive-id，并提示 root 选择或接受显式覆盖参数，全程不打印 token

#### Scenario: Non-interactive space creation
- **WHEN** 在非交互环境（管道/MCP 宿主）创建空间且未提供 root-id 或 root-name
- **THEN** 拒绝并返回固定错误码，不进入交互等待或猜测目录

#### Scenario: Honest auth status
- **WHEN** 查询 auth 状态而未显式探测
- **THEN** 只报告本地可判定的 token 状态与 not probed 的账号可用性，不以配置存在冒充已登录
