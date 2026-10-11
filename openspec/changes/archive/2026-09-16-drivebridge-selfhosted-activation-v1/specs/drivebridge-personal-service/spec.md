# Delta: drivebridge-personal-service

## ADDED Requirements

### Requirement: Key-only activation rehearsal
服务端 SHALL 提供一条离线可重复的 key-only 激活彩排路径：以真实 `drivebridge-server` 二进制、真实网页构建产物与回环 S3 fixture 走通 platform-connect → users import → serve --enabled → status 远程核对序列，并覆盖浏览器等价冒烟（登录方式如实报告、密钥换会话、建空间、上传/下载小文件、配额口径）与备份只读校验；每次彩排 SHALL 按集成测试证据标准落盘，明文密钥只出现在 0600 权限的 artifacts 分发文件。

#### Scenario: Four-step key-only rehearsal passes offline
- **WHEN** 操作者运行彩排脚本且无外部网络与真实凭据
- **THEN** 四步序列与冒烟全部通过，证据目录完整；任一步失败以非零退出并保留证据

#### Scenario: Fixture S3 is explicit and loopback-only
- **WHEN** 以 --s3-listen 启动 fixture
- **THEN** S3 endpoint 独立监听在指定回环地址并输出到 stderr；不带该旗标时 fixture 行为与之前一致

## MODIFIED Requirements

### Requirement: Configured deployment surface
服务端 CLI SHALL 支持可选部署配置文件与命令树：发现序为显式路径、环境变量、工作目录、用户配置目录、系统配置目录；解析优先级为显式旗标、环境变量、文件、内置默认；未知配置键与不合形状的值 SHALL 被拒绝而不是静默忽略。CLI 失败 SHALL 经输出机制渲染真实类型码（机器模式出合法失败信封）；audit 人读模式 SHALL 打印事件行；doctor 未就绪以退出码 2 与硬错误区分。`serve` SHALL 额外支持 `--events` 监督流：start（含 spec_version）→ready（仅在监听器真实绑定后）→end/error，其余输出模式仍被拒绝、日志仍走 stderr。`platform-connect` SHALL 支持显式 `--development` 旗标以允许回环 HTTP fixture 端点；该开关只能来自显式命令行，不进入配置文件与环境变量，缺省时平台存储端点仍要求 HTTPS 公网。

#### Scenario: Deployment from config file
- **WHEN** 部署者以配置文件提供 state/listen/origin/issuer/secret 文件路径等部署参数并以 `serve --enabled` 启动
- **THEN** 服务按配置解析并启动，等效于等价的全旗标命令行

#### Scenario: Write switches and secrets stay explicit
- **WHEN** 解析任何配置来源
- **THEN** 默认关闭的入口开关（enabled/development）与凭据值不可来自文件或环境；文件只承载路径、URL 与限额

#### Scenario: Invalid config fails loudly
- **WHEN** 配置文件含白名单外键、带路径的 origin 或非正整数 max_bytes
- **THEN** 命令以错误退出并指出键名，不静默忽略或回退默认

#### Scenario: Machine-mode failure envelopes
- **WHEN** 任一 CLI 命令在 --json/--agent/--events 下失败
- **THEN** stdout 收到携带真实错误码的合法失败投影，诊断只在 stderr

#### Scenario: Audit rows in human mode
- **WHEN** 以默认人读模式运行 audit list
- **THEN** 事件行（时间/动作/actor/结局/资源）直接可见，不必切 --json

#### Scenario: Not-ready is exit code 2
- **WHEN** doctor 汇报未就绪
- **THEN** 以退出码 2 结束（与硬错误 1 区分），报告已是完整投影

#### Scenario: Serve supervision stream
- **WHEN** 以 --events 运行 serve
- **THEN** stdout 流出 start→progress(stage=ready，监听器已绑定)→end/error；--json/--agent/--explain 仍被拒绝且日志只在 stderr

#### Scenario: platform-connect loopback rehearsal endpoint
- **WHEN** 以 --development 运行 platform-connect 且 stdin 端点为回环 HTTP fixture
- **THEN** 连接被接受并按既有流程探测；不带 --development 时同一端点仍以原有错误码拒绝
