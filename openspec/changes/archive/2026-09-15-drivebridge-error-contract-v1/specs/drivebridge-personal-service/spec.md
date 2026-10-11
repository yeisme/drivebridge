## MODIFIED Requirements

### Requirement: Safe user and Agent interfaces
系统 SHALL 通过统一服务提供网页/API/CLI/MCP，临时凭据不得进入普通 MCP 结果和日志。HTTP 失败信封 SHALL 携带 `error.code` 与静态英文 `message`（注册表驱动，不插值请求或原始错误文本）；状态码映射 SHALL 与失败类别一致（鉴权 401、不存在 404、冲突 409、限流 429、服务端故障 503）；SDK 错误 SHALL 可类型化读取 code 与 message 且 `Error()` 保持裸码字节兼容。

#### Scenario: Remote client path
- **WHEN** 远程 MCP 收到客户端文件路径
- **THEN** 不把路径当作远端文件，指引宿主通过 HTTP 传输字节

#### Scenario: User changes file version
- **WHEN** 当前版本与调用者指定版本不同
- **THEN** 不覆盖文件并返回版本冲突

#### Scenario: Failure envelope carries a message
- **WHEN** 任何 /api/v1 请求失败
- **THEN** 信包含非空静态 message；第三方文本不进入 code 或 message；服务端故障码映射 5xx、限流 429、冲突 409 而非一律 400

#### Scenario: SDK error stays byte-compatible
- **WHEN** SDK 请求失败
- **THEN** err.Error() 返回裸码不变；类型断言可读取 message（服务端省略时为空）

### Requirement: Operator-issued user keys
系统 SHALL 支持操作员在本地终端批量签发或导入用户密钥以创建用户身份：密钥明文仅在签发输出出现一次，存储只保留哈希，生命周期可查询与吊销；用户密钥身份不经 OAuth/OIDC 自助注册。重复哈希 SHALL 以 `user_key_duplicate` 报告且整批不落库；数据库失败 SHALL 以 `database_unavailable` 报告，不共用同一码。

#### Scenario: Batch import generates keys
- **WHEN** 操作员经 CLI 提交 N 条不带密钥的用户条目
- **THEN** 系统为每条生成 `dbu_` 格式密钥并创建用户，明文密钥只在本次签发输出出现一次

#### Scenario: Operator-supplied keys
- **WHEN** 条目携带操作员自备密钥
- **THEN** 系统校验格式与唯一性后导入，格式不符或哈希重复时整批拒绝且不落任何新用户

#### Scenario: Revoked key rejected
- **WHEN** 已吊销或已过期的用户密钥发起认证
- **THEN** 返回 unauthorized，不区分"不存在"与"已吊销"的响应体

### Requirement: Deployment preflight
doctor SHALL 以只读方式报告部署就绪项，按名区分 pass/fail/info；不得获取实例锁、修改状态或输出凭据；state 不一致时给出恢复指引而不是重新生成的暗示。可登录身份 SHALL 预检：已建库但不存在任何未吊销未过期用户密钥且未配置 OIDC issuer 时按名失败并给出恢复命令。

#### Scenario: Consistent existing state
- **WHEN** state 三件（instance.key、instance.id、metadata.sqlite）一致且平台存储已配置
- **THEN** 相关检查项汇总 pass，命令以 0 退出

#### Scenario: Key or instance identity missing while database exists
- **WHEN** metadata.sqlite 存在但 instance.key 或 instance.id 缺失/形状不符
- **THEN** 对应检查项按名失败并指向秘密备份恢复路径，不以非 0 之外的任何方式掩盖或触发重新生成

#### Scenario: Optional issuer probe failure does not block
- **WHEN** 传入 `--issuer` 且 discovery 端点不可达或 issuer 字段不匹配
- **THEN** 仅 oidc_issuer 项失败，其余检查照常报告，命令整体 not ready

#### Scenario: No loginable identity
- **WHEN** 已有状态库但无未吊销未过期用户密钥且未配置 issuer
- **THEN** login_identities 检查失败并指向 users import 或 OIDC 配置；配置了 issuer 时降为 info

### Requirement: Storage isolation verification probe
`platform-connect` SHALL 提供隔离验证探针：仅尝试在保留探针命名空间写入自身命名空间之外的对象并期待拒绝；意外成功须立即清理并在结果中报告凭据超域；探针不得触碰真实租户命名空间。`--verify-isolation` SHALL 折叠进单一 `platform-connect` 信封：隔离结果承载于 `data.storage_isolation`，超域时信封 status 为 partial。

#### Scenario: Prefix-scoped credential
- **WHEN** 凭据被云侧策略收窄到本部署命名空间
- **THEN** 两处探针写入均被拒绝，报告 isolated

#### Scenario: Over-scoped credential
- **WHEN** 凭据可写桶内任意位置或前缀下兄弟命名空间
- **THEN** 探针成功后立即删除该临时对象并报告超域，不以成功冒充隔离

#### Scenario: Probe containment
- **WHEN** 探针执行
- **THEN** 写入仅发生在保留探针命名空间，且所有成功写入都被删除

#### Scenario: One envelope per command
- **WHEN** 带 --verify-isolation 以 --json 运行
- **THEN** stdout 恰一个信封：status=success（isolated）或 partial（over_scoped），storage_isolation 在 data 内

### Requirement: Configured deployment surface
服务端 CLI SHALL 支持可选部署配置文件与命令树：发现序为显式路径、环境变量、工作目录、用户配置目录、系统配置目录；解析优先级为显式旗标、环境变量、文件、内置默认；未知配置键与不合形状的值 SHALL 被拒绝而不是静默忽略。CLI 失败 SHALL 经输出机制渲染真实类型码（机器模式出合法失败信封）；audit 人读模式 SHALL 打印事件行；doctor 未就绪以退出码 2 与硬错误区分。

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
