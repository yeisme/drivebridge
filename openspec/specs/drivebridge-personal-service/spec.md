# drivebridge-personal-service Specification

## Purpose
TBD - created by archiving change drivebridge-personal-service-v1. Update Purpose after archive.
## Requirements
### Requirement: Personal identity isolation
系统 SHALL 校验用户、Agent 和资源归属，OIDC 登录不自动授予 OneDrive 文件访问。

#### Scenario: Another user requests a file
- **WHEN** 用户请求他人的文件、版本、操作或连接
- **THEN** 返回拒绝且不泄露内容或存在性

#### Scenario: Agent authority revoked
- **WHEN** Agent token 已撤销、过期或超出目录/操作范围
- **THEN** 拒绝后续请求

### Requirement: Durable file transfer

系统 SHALL 保留稳定文件身份、不可变版本和原操作恢复。当授权范围内已存在相同明文 SHA-256 的 blob 时，系统 SHALL 发布新 version 指向该 blob，SHALL NOT 再写一份对象。未命中时 SHALL 保持原分片上传与复读校验。

#### Scenario: Interrupted upload
- **WHEN** 分片上传中断或服务重启
- **THEN** 查询原 operation 及已接收分片，校验相同源文件后继续，不创建重复文件

#### Scenario: Replayed complete
- **WHEN** 客户端重放已完成操作
- **THEN** 返回同一文件版本与回执

#### Scenario: Authorized blob reuse
- **WHEN** 同一 data owner 再次上传已存储的相同 SHA-256 到新路径
- **THEN** 操作完成且不写新对象，新旧 version 均可按原 URI 读取相同字节

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

### Requirement: Deployment tenant boundary
未显式启用治理的部署 SHALL 保持实例即租户；通过部署侧治理引导后 SHALL 支持单实例多租户，并逐请求检查租户成员关系与资源归属。实例身份继续表示引用所属部署，不等同于已启用治理后的业务租户 ID。

#### Scenario: Cross-instance reference
- **WHEN** 请求携带的引用实例身份与当前部署不匹配
- **THEN** 返回拒绝且不泄露目标内容或存在性

#### Scenario: Tenant profile is read-only metadata
- **WHEN** 旧部署配置租户名称等 profile 后在线查询
- **THEN** 既有 profile 仍为只读部署元数据，不获得新的治理角色

#### Scenario: Multi-tenant scaling path
- **WHEN** 需要服务多个租户且治理未启用
- **THEN** 按实例分片部署（独立 state/origin/issuer），不承诺单实例内租户化或跨实例数据迁移

#### Scenario: Explicit multi-tenant activation
- **WHEN** 部署操作者指定首批最高管理员并启用治理
- **THEN** 旧数据进入默认租户，其他租户通过独立成员关系隔离，原引用与文件 ID 保留

### Requirement: Backup verification and object manifest
备份 SHALL 附带对象清单并支持只读校验：清单与元数据交叉一致、抽样对象存在性可探测；校验失败按名报告，不宣称恢复就绪。

#### Scenario: Manifest matches metadata and objects
- **WHEN** 对一份备份执行校验且清单、元数据与抽样对象全部一致
- **THEN** 报告通过并给出抽样统计，恢复演练可继续

#### Scenario: Manifest or object drift detected
- **WHEN** 清单条目与元数据不一致或抽样对象缺失
- **THEN** 以固定错误码按名报告失败项，整体判定不可信，不给部分通过的恢复承诺

#### Scenario: Restore drill verification
- **WHEN** 在隔离目录完成恢复后执行只读校验
- **THEN** 校验覆盖备份三件一致性与清单交叉，密钥取回步骤包含在演练内

### Requirement: Content envelope encryption
加密空间 SHALL 以实例密钥包裹的 per-space 密钥做服务端信封加密：对象落盘为密文、明文摘要保留在元数据、解密仅发生在服务端授权路径；加密状态创建后不可变。

#### Scenario: Encrypted space roundtrip
- **WHEN** 向 opt-in 加密空间上传并随后下载或预览
- **THEN** 存储侧字节为密文，返回内容与明文摘要一致，密钥材料不进入输出、日志或回执

#### Scenario: Instance key lost
- **WHEN** 加密空间的实例密钥缺失（恢复场景不满足三件一致）
- **THEN** 拒绝服务并明确指向密钥备份，不得以重新生成密钥掩盖（与现有恢复纪律一致）

#### Scenario: Direct-link narrowed for encrypted spaces
- **WHEN** 对加密空间请求私有下载授权或参考交接
- **THEN** 不返回对象存储直链；下载经短时服务端解密流，OneDrive 参考交接明确拒绝并说明原因

#### Scenario: Key rotation without object rewrite
- **WHEN** 操作者轮换实例密钥
- **THEN** 仅重新包裹空间密钥，不重写对象字节，新旧版本可读性保持

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

### Requirement: S3-compatible virtual-hosted addressing
平台存储与 S3 连接 SHALL 支持显式虚拟主机寻址，以兼容拒绝 path-style 的 S3 兼容提供方（如腾讯云 COS）；未声明时保持既有 path-style 行为，已封存连接零迁移。

#### Scenario: Virtual-hosted platform storage on COS
- **WHEN** 平台存储以 `virtual_hosted` 声明配置在拒绝 path-style 的提供方
- **THEN** 探测、写入、读取、校验与删除经 `<bucket>` 子域主机完成，失败不泄露凭据或签名材料

#### Scenario: Legacy sealed connection
- **WHEN** 打开不带 `virtual_hosted` 声明的既有封存连接
- **THEN** 仍按 path-style 寻址，行为与本变更前一致

### Requirement: Shared-bucket tenant namespace
共享桶多部署 SHALL 保持每部署独立对象命名空间：平台存储对象键恒以部署实例标识开头；该隔离是命名空间级，字节级访问控制依赖前缀作用域子凭据或空间信封加密，系统不得声称更强的隔离。

#### Scenario: Instance-scoped object keys
- **WHEN** 文件经平台存储写入共享桶
- **THEN** 对象键以本部署实例标识开头，不含其他部署命名空间

#### Scenario: Honest trust boundary
- **WHEN** 多个部署共享同一存储桶或存储凭据
- **THEN** 文档与验证面如实说明存储层无字节级隔离，并指向云侧前缀作用域子凭据与应用侧信封加密两条缓解路径

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

### Requirement: Operator-issued user keys
系统 SHALL 支持操作员在本地终端批量签发或导入用户密钥以创建用户身份：密钥明文仅在签发输出出现一次，认证保留哈希，并另存绑定用户与 Key ID 的加密密文供本人再次取回，生命周期可查询与吊销；用户密钥身份不经 OAuth/OIDC 自助注册。重复哈希 SHALL 以 `user_key_duplicate` 报告且整批不落库；数据库失败 SHALL 以 `database_unavailable` 报告，不共用同一码。

#### Scenario: Batch import generates keys
- **WHEN** 操作员经 CLI 提交 N 条不带密钥的用户条目
- **THEN** 系统为每条生成 `dbu_` 格式密钥并创建用户，明文密钥只在本次签发输出出现一次

#### Scenario: Operator-supplied keys
- **WHEN** 条目携带操作员自备密钥
- **THEN** 系统校验格式与唯一性后导入，格式不符或哈希重复时整批拒绝且不落任何新用户

#### Scenario: Revoked key rejected
- **WHEN** 已吊销或已过期的用户密钥发起认证
- **THEN** 返回 unauthorized，不区分"不存在"与"已吊销"的响应体

### Requirement: Key-only deployment
部署 SHALL 支持无 issuer 的 key-only 模式：`serve --enabled` 不因缺少 OIDC 配置拒绝启动，浏览器登录如实报告 oidc_not_configured，Bearer 用户密钥照常认证。

#### Scenario: Boot without issuer
- **WHEN** 以 `--enabled` 启动且未配置 issuer/client-id
- **THEN** 服务正常监听，密钥 Bearer 请求按正常权限处理

#### Scenario: Browser login honestly rejected
- **WHEN** key-only 部署下访问浏览器登录入口
- **THEN** 返回 oidc_not_configured，不降级为其他登录方式

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

### Requirement: Scoped operation inbox
系统 SHALL 提供收件箱查询：列出调用者近 30 天自有操作，支持 status/caller/space 可选过滤，且仅返回当前身份对操作目标具备读权限的条目；数据库不可用时如实失败。

#### Scenario: Permission filtered after limit
- **WHEN** 操作归属调用者但当前身份已失去对目标空间或路径的读权限
- **THEN** 该条目不出现在结果中，返回数量可以少于请求的 limit

#### Scenario: Honest failure
- **WHEN** 状态数据库查询失败
- **THEN** 返回 database_unavailable，不以空列表冒充成功

### Requirement: Managed version archive
系统 SHALL 支持把指定固定版本经服务端复制进受管目标空间：以幂等键去重，绝不覆盖既有文件，源版本与可选期望头版本冲突即拒绝；归档作为可恢复操作保留来源与 provenance。目标空间缺乏可用受管 S3 存储时，系统 SHALL 在任何字节流动、配额预留或操作创建之前返回 unsupported。

#### Scenario: Never replaces an existing file
- **WHEN** 目标路径已存在未回收文件
- **THEN** 归档落在自动改名的新路径（原名加序号），既有文件保持不变

#### Scenario: Pinned and conflict-checked source
- **WHEN** 指定版本不属于源文件，或期望头版本与源当前头不一致
- **THEN** 返回 not_found 或 version_conflict，不产生任何传输

#### Scenario: Idempotent replay and resume
- **WHEN** 同一归档请求以相同幂等键重放，或归档中断后恢复
- **THEN** 幂等命中返回既有操作；恢复按记录的源固定版本重新传输字节，不生成新内容

#### Scenario: Non-managed destination rejected before transfer
- **WHEN** 目标空间的存储不是启用中的 S3 连接（缺失、禁用或非 s3 类别）
- **THEN** 返回 unsupported，不创建操作、不预留配额、不传输任何字节

### Requirement: Key exchange session login
key-only 部署 SHALL 提供密钥换发会话登录：用户提交一次用户密钥换取与 OIDC 同构的 cookie 会话；浏览器不存储密钥，登出或过期即失效。

#### Scenario: Valid key exchanged for session
- **WHEN** 有效用户密钥提交到密钥登录入口
- **THEN** 服务设置会话 cookie 并可像 OIDC 会话一样访问全部用户接口

#### Scenario: Invalid or revoked key
- **WHEN** 提交错误、已吊销或已过期的密钥
- **THEN** 返回 unauthorized，不区分不存在与已吊销

#### Scenario: No token persists in browser
- **WHEN** 密钥换发会话完成后检查浏览器存储
- **THEN** 密钥不出现在任何客户端存储，仅存在服务端会话 cookie

### Requirement: Admin session inheritance
未启用治理的部署中，admin 用户密钥会话 SHALL 保留旧权限投影；启用治理后 SHALL 以实时平台及租户角色为权限来源，不把旧会话 admin 标记自动升级。

#### Scenario: Admin session reaches management surface
- **WHEN** admin 密钥换发的会话调用用户管理接口
- **THEN** 与 admin 密钥 Bearer 同权限

#### Scenario: Plain session rejected
- **WHEN** 非 admin 会话或非 admin 密钥调用用户管理接口
- **THEN** 返回 forbidden

#### Scenario: Legacy deployment
- **WHEN** 未启用治理的 admin 会话调用管理面
- **THEN** 与旧 admin 密钥 Bearer 同权限

#### Scenario: Governance activation
- **WHEN** 治理已启用，旧 admin 会话没有显式角色
- **THEN** 不获得管理读取或应急权限

### Requirement: User management HTTP surface
系统 SHALL 保留旧列表响应形状且不返回凭据或哈希；治理启用后列表仅平台最高管理员可读，旧签发、轮换及吊销管理写接口 SHALL 拒绝并引导到账号激活、恢复与冻结流程。

#### Scenario: Admin lists users
- **WHEN** admin 调用用户列表接口
- **THEN** 返回全部用户密钥的生命周期元数据（id/用户/名称/admin/过期/吊销），不含哈希或明文密钥

#### Scenario: Admin revokes a key
- **WHEN** admin 调用吊销接口指定密钥 id
- **THEN** 该密钥下一次认证被拒，接口返回成功

#### Scenario: Legacy admin revokes a key
- **WHEN** 未启用治理的 admin 调用旧吊销接口
- **THEN** 该密钥下一次认证被拒，原合同保持不变

#### Scenario: Governance recovery boundary
- **WHEN** 已启用治理的管理员试图通过旧管理写接口取得他人新密钥
- **THEN** 返回 account_recovery_required，不执行写入

### Requirement: Honest login mode reporting
系统 SHALL 如实报告本部署可用的登录方式（OIDC 与/或密钥），该报告无需认证且不泄露租户敏感信息。

#### Scenario: Key-only deployment
- **WHEN** 未配置 issuer 的部署查询登录方式
- **THEN** 报告 oidc=false、key=true

### Requirement: Tabular batch issuance
`users import` SHALL 接受 CSV 批量输入并可选写出分发文件：带表头（name/days/admin/key）或按位无表头，单列即纯名单；分发文件只含签发输出且权限 0600；JSON 输入路径行为不变。

#### Scenario: Header CSV with per-user options
- **WHEN** 提交带 name,days,admin 表头的 CSV
- **THEN** 按列解析批量签发，格式或重复错误整批拒绝零落库

#### Scenario: Plain name list
- **WHEN** 提交单列名单（无表头、容忍空行与 BOM）
- **THEN** 每行作为一个用户签发，默认长期有效非 admin

#### Scenario: Distribution file
- **WHEN** 指定 --out 分发文件
- **THEN** 写出 name,user_id,key,admin,expires_at 的 CSV 且文件权限 0600

#### Scenario: JSON path unchanged
- **WHEN** 不带 --csv 提交 stdin JSON
- **THEN** 行为与本变更前一致

### Requirement: External pre-generated key import
系统 SHALL 支持导入外部预生成密钥：去空白后长度 32–128 的可打印 ASCII 均可导入（任意前缀）；与既有用户密钥或 Agent 令牌哈希冲突时整批拒绝零落库；自生成密钥保持 dbu_ 格式不变。

#### Scenario: Foreign-format key imported
- **WHEN** 操作员导入外部工具生成的密钥（非 dbu_ 前缀、可打印且长度达标）
- **THEN** 导入成功，该密钥经 Bearer 与密钥登录均按用户身份认证

#### Scenario: Entropy floor enforced
- **WHEN** 导入短于 32 字符、含空白或超过 128 字符的密钥
- **THEN** 整批拒绝且不落任何新用户

#### Scenario: Cross-table collision rejected
- **WHEN** 导入密钥的哈希与任一 Agent 令牌或既有用户密钥相同
- **THEN** 整批拒绝

### Requirement: Key rotation
部署侧密钥原子换发 SHALL 保留同一用户身份及旧密钥失效语义；治理启用后的在线恢复 SHALL 由本人领取新凭据，管理员不能通过旧在线轮换接口取得密钥。

#### Scenario: Rotate keeps identity
- **WHEN** 管理员轮换某用户的密钥
- **THEN** 新密钥立即以同一用户身份工作，旧密钥立即失效，用户与其数据归属不变

#### Scenario: Operator recovery
- **WHEN** 部署操作者在本机执行明确的密钥恢复
- **THEN** 用户与数据归属不变，不将该部署侧信任边界伪装成普通 Web 权限

#### Scenario: Owner claims recovered credential
- **WHEN** 本人提交有效领取证明且恢复已获批
- **THEN** 旧密钥、会话及未完成授权失效，人工恢复的特权需独立确认

### Requirement: Operator status projection
服务端 CLI SHALL 提供只读 `status` 命令：远程模式以 0600 用户密钥文件探测运行中部署的 healthz、身份与空间并输出单一投影；本地模式以只读快照投影 state 完整性、空间数、配额合计与用户密钥计数；两种模式的人读摘要 SHALL 携带恰好一条下一步建议，且不输出凭据。

#### Scenario: Remote projection
- **WHEN** 以 --url 与 --token-file 运行 status
- **THEN** healthz 可达性、token 对应身份（含 admin）与空间列表折叠进一个信封；token 文件组/其他可读时拒绝

#### Scenario: Local snapshot
- **WHEN** 无 --url 运行 status
- **THEN** 只读快照输出 state 三件存在性、空间计数、max/used/reserved 合计与 user key 总数/存活数；未初始化的部署如实报告并指向初始化下一步

#### Scenario: Typed failure per hop
- **WHEN** healthz 不可达、token 被拒或空间投影失败
- **THEN** 分别以 service_unreachable、token_rejected、service_request_failed 失败，不冒充成功

### Requirement: Operator agent token issuance
服务端 CLI SHALL 支持操作员在私有状态目录直签 Agent 令牌：owner 从目标空间行推导，校验与令牌铸造与 HTTP 面同路；明文令牌只在签发输出出现一次（stdout 或 0600 文件二选一）；列表仅含生命周期元数数据。

#### Scenario: Owner derived from the space
- **WHEN** 操作员对已知空间执行 agents create
- **THEN** 令牌归属该空间属主，校验（名称/前缀/scopes/days）与 HTTP 面一致；未知空间返回 not_found

#### Scenario: One-shot token discipline
- **WHEN** 使用 --out 签发
- **THEN** 令牌写入 0600 文件且信封不携带明文；不带 --out 时 stdout 打印一次并提示即刻保存

#### Scenario: List never surfaces secrets
- **WHEN** 执行 agents list
- **THEN** 只读快照输出 id/name/space/scopes/expires/revoked，哈希与令牌永不出现

### Requirement: Session revocation cascade
密钥换发会话 SHALL 跟随来源密钥生命周期：来源密钥被吊销或过期后，会话下一次认证即失效；非密钥来源会话不受影响。本人通过专用接口确认轮转时，SHALL 仅将当前操作会话的来源迁移至新 Key。

#### Scenario: Revoked key kills its sessions
- **WHEN** 用户密钥被吊销或轮换后，除本人轮转时已迁移的当前会话外，其余换发会话发起请求
- **THEN** 返回 unauthorized，会话立即失效

#### Scenario: OIDC sessions unaffected
- **WHEN** 任一用户密钥被吊销
- **THEN** OIDC 登录建立的会话不受影响

### Requirement: Operator key verification
服务端 CLI SHALL 支持以明文核对一把已分发用户密钥：本地哈希查找，零网络流量、零认证副作用；报告 id/name/admin/过期时间与状态（live、revoked、expired），未知密钥返回 not_found。

#### Scenario: Verify a distributed key
- **WHEN** 操作员对签发输出中的明文密钥执行 users verify --key
- **THEN** 报告该密钥的当前生命周期状态；吊销后的同一密钥报告 revoked，过期报告 expired

#### Scenario: Unknown key is not_found
- **WHEN** 核对一把不存在的密钥
- **THEN** 返回 not_found；核对过程不产生登录事件或网络请求

### Requirement: Running-deployment key rotation
admin SHALL 可经 HTTP 轮换用户密钥（运行中部署无需停服窗口）；已吊销或已过期的密钥不可轮换。

#### Scenario: Admin rotates over HTTP
- **WHEN** admin 调用密钥轮换接口
- **THEN** 新密钥一次性返回，旧密钥立即失效，吊销级联语义同 CLI

#### Scenario: Non-admin rejected
- **WHEN** 非 admin 凭据调用轮换接口
- **THEN** 返回 forbidden

#### Scenario: Expired key not rotatable
- **WHEN** 对已过期或已吊销的密钥发起轮换
- **THEN** 返回 not_found，不产生出生即死的新密钥

### Requirement: Key-only activation rehearsal
服务端 SHALL 提供一条离线可重复的 key-only 激活彩排路径：以真实 `drivebridge-server` 二进制、真实网页构建产物与回环 S3 fixture 走通 platform-connect → users import → serve --enabled → status 远程核对序列，并覆盖浏览器等价冒烟（登录方式如实报告、密钥换会话、建空间、上传/下载小文件、配额口径）与备份只读校验；每次彩排 SHALL 按集成测试证据标准落盘，明文密钥只出现在 0600 权限的 artifacts 分发文件。

#### Scenario: Four-step key-only rehearsal passes offline
- **WHEN** 操作者运行彩排脚本且无外部网络与真实凭据
- **THEN** 四步序列与冒烟全部通过，证据目录完整；任一步失败以非零退出并保留证据

#### Scenario: Fixture S3 is explicit and loopback-only
- **WHEN** 以 --s3-listen 启动 fixture
- **THEN** S3 endpoint 独立监听在指定回环地址并输出到 stderr；不带该旗标时 fixture 行为与之前一致

