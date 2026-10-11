## ADDED Requirements

### Requirement: Operator key verification
服务端 CLI SHALL 支持以明文核对一把已分发用户密钥：本地哈希查找，零网络流量、零认证副作用；报告 id/name/admin/过期时间与状态（live、revoked、expired），未知密钥返回 not_found。

#### Scenario: Verify a distributed key
- **WHEN** 操作员对签发输出中的明文密钥执行 users verify --key
- **THEN** 报告该密钥的当前生命周期状态；吊销后的同一密钥报告 revoked，过期报告 expired

#### Scenario: Unknown key is not_found
- **WHEN** 核对一把不存在的密钥
- **THEN** 返回 not_found；核对过程不产生登录事件或网络请求

## MODIFIED Requirements

### Requirement: Configured deployment surface
服务端 CLI SHALL 支持可选部署配置文件与命令树：发现序为显式路径、环境变量、工作目录、用户配置目录、系统配置目录；解析优先级为显式旗标、环境变量、文件、内置默认；未知配置键与不合形状的值 SHALL 被拒绝而不是静默忽略。CLI 失败 SHALL 经输出机制渲染真实类型码（机器模式出合法失败信封）；audit 人读模式 SHALL 打印事件行；doctor 未就绪以退出码 2 与硬错误区分。`serve` SHALL 额外支持 `--events` 监督流：start（含 spec_version）→ready（仅在监听器真实绑定后）→end/error，其余输出模式仍被拒绝、日志仍走 stderr。

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
