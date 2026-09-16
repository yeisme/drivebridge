# drivebridge-local-files Specification

## Purpose
TBD - created by archiving change drivebridge-local-mcp-v1. Update Purpose after archive.
## Requirements
### Requirement: Stable identity and private transport
系统 SHALL 使用稳定文件引用并隔离空间、身份、目录和文件版本，临时下载 URL 只交给已配置的本机 owner adapter。

#### Scenario: Allowed reference
- **WHEN** 原任务身份有权消费指定空间的固定版本文件
- **THEN** 系统返回稳定引用与校验摘要，在实际提交前通过私有通道获取下载授权

#### Scenario: Wrong identity or replaced file
- **WHEN** 访问者身份、空间或文件版本不匹配
- **THEN** 系统拒绝交接，不返回下载 URL

### Requirement: Recovery does not repeat generation
系统 SHALL 使用原操作/原任务进行恢复，禁止因网络错误或回存失败自动发起新的生成。

#### Scenario: Lost upload acknowledgement
- **WHEN** 上传已完成但客户端没有取得回执
- **THEN** 恢复核对原操作的远端文件摘要并返回同一文件，不重复创建资产

#### Scenario: Output save fails
- **WHEN** 成功生成任务的成品回存失败
- **THEN** 只恢复文件传输，保留原领域任务与审核状态

### Requirement: Explicit activation and honest readiness
系统 SHALL 默认关闭新增写入口，并区分本地 fixture 验证与真实账号、Gateway、Provider 区域验收。不带 `--enabled` 启动 MCP 服务时 SHALL 在启动即向 stderr 告知被门禁的工具面；进程锁冲突 SHALL 以类型化 `state_locked` 报告而非原始系统错误。

#### Scenario: Secure handoff absent
- **WHEN** 宿主没有可用的私有交接通道
- **THEN** 系统明确拒绝参考交接，不把 URL 写入普通 MCP 输出，也不自动配置 TOS

#### Scenario: Startup warning without enabled
- **WHEN** 不带 --enabled 启动 drivebridge mcp
- **THEN** stderr 立即警告写工具将返回 disabled（--handoff 时并注明其依赖 --enabled），stdout 保持 stdio MCP 专用

#### Scenario: Typed lock conflict
- **WHEN** 另一进程（典型为已注册 MCP 客户端）持有 state 的进程锁
- **THEN** 一次有界短重试后以 state_locked 失败，提示点名持锁者与快照读出路，不暴露原始 OS 错误文本

### Requirement: Preflight readiness reporting
doctor SHALL 以只读方式报告激活所需预检项，按名区分 pass/fail/info；不得获取写锁、发起真实账号探测，或在输出中包含凭据、临时 URL 或原始第三方错误。handoff socket 的重启 SHALL 仅在旧 socket 被探测确认死亡后自动回收；state 目录缺失的提示 SHALL 给出确切创建命令。

#### Scenario: All prerequisites met
- **WHEN** 本机预检全部通过（私有 state、rclone 远端、空间、可写 local root、ffprobe，及可选的 handoff socket 与 Scaena 端点）
- **THEN** 命令汇总 ready 并以 0 退出

#### Scenario: Missing or broken prerequisite
- **WHEN** 任一预检项缺失或损坏（无空间、local root 不可写、残留 socket、凭据文件权限过宽等）
- **THEN** 按名报告失败项并以非 0 退出，明细不泄露 token、URL 或原始错误正文

#### Scenario: Remote mode preflight
- **WHEN** 以 `--remote-url` 运行 doctor
- **THEN** 校验服务地址形状、token 文件权限与 local root，并探测 `/healthz` 可达性；可达不代表授权有效，授权状态记为 not probed

#### Scenario: Stale socket self-heal
- **WHEN** handoff socket 因异常退出残留且绑定时仍有同名文件
- **THEN** 仅当 dial-probe 确认无消费者时回收并重试一次；活/未知 socket 维持显式清理规则

#### Scenario: Exact recovery command for missing state
- **WHEN** doctor 在全新机器上未发现 state 目录
- **THEN** 提示给出确切的 `mkdir -m 0700` 命令与等价出路，doctor 自身保持只读

### Requirement: Backend kind capability matrix
空间 SHALL 声明后端类别并在能力面如实区分传输能力与 OneDrive 专属能力；非 OneDrive 后端不得暴露临时直链或参考交接。

#### Scenario: Generic rclone backend space
- **WHEN** 空间以 generic 后端（如 SMB/WebDAV/S3/local）配置
- **THEN** list/stat/upload/download 与操作恢复可用，`prepare_reference` 与直链获取返回固定拒绝码，不降级为其他交付方式

#### Scenario: Honest capability reporting
- **WHEN** 能力资源或 tools/list 描述空间
- **THEN** 按 kind 报告 transfer/direct_url/reference_handoff 能力，未配置的后端不虚构支持

### Requirement: Symbolic link policy
传输与同步 SHALL 默认拒绝符号链接以防越界逃逸；仅空间级显式 opt-in 后以物化拷贝跟随。

#### Scenario: Symlink encountered by default
- **WHEN** 上传或同步扫描遇到符号链接（含指向 local root 之外）
- **THEN** 以固定错误码拒绝该项，不跟随、不跳过不报，也不写入对象

#### Scenario: Explicit follow opt-in
- **WHEN** 空间显式启用 follow-symlinks
- **THEN** 链接以内容副本物化上传，对象端不保留链接语义，opt-in 状态在能力面可见

### Requirement: Configured CLI surface
CLI SHALL 支持可选配置文件与命令树：发现序为显式路径、环境变量、工作目录、用户配置目录；解析优先级为显式旗标、环境变量、文件、内置默认；未知配置键 SHALL 被拒绝而不是静默忽略。新建配置文件的默认文件名 SHALL 统一（`config.yaml`），无论由 `config init` 还是 `config set` 创建；既有其他扩展名的文件继续可读。

#### Scenario: Discovery and precedence
- **WHEN** 存在配置文件且同一键同时有环境变量与显式旗标
- **THEN** 显式旗标胜出，其次环境变量，其次文件，且 `config show` 如实报告每键来源

#### Scenario: Unknown configuration key
- **WHEN** 配置文件包含白名单之外的键
- **THEN** 命令以错误退出并列出未知键名，不静默忽略或回退默认

#### Scenario: Security switches stay explicit
- **WHEN** 解析任何配置来源
- **THEN** 默认关闭的写入口开关（enabled/handoff）与凭据值不可来自文件或环境，只能显式命令行；配置面只承载路径与 URL

#### Scenario: One default config filename
- **WHEN** 用户目录尚无配置文件时执行 config set
- **THEN** 新文件落为 config.yaml（与 config init 一致）；历史 config.json 仍按发现序读取

### Requirement: Workspace directories and transfer CLI
工作区 SHALL 支持目录组织与直连 CLI：目录参数限定在空间钉定根内，传输与组织复用既有操作/恢复/引用合同；CLI 是用户本人终端的等价入口。同步等待传输的 CLI 命令 SHALL 在 `--events` 下流式输出 progress 事件（seq 单调、节流），人读模式在 stderr 刷新单行进度；`--agent` 投影 SHALL 将嵌套 map 扁平化一层并以计数表达切片。索引检索（find）与事件流（changes）SHALL 在排他进程锁被持有时回退只读快照并以 `read_only` 标注，快照命中保持 advisory。

#### Scenario: Organize and transfer by directory
- **WHEN** agent 或用户在空间内创建目录并指定目录上传、按目录列出
- **THEN** 目标名保持操作前缀（不替换既有文件），按路径列出的引用与根层引用同一稳定合同

#### Scenario: Path escape rejected
- **WHEN** 目录参数为绝对路径、含 .. 遍历或未清洗形式
- **THEN** 返回固定拒绝码，不触达空间外的远端路径

#### Scenario: mkdir idempotence
- **WHEN** 重复创建已存在的目录路径
- **THEN** 成功返回；同名文件占用该段时返回冲突而非覆盖

#### Scenario: CLI retry reuses the original operation
- **WHEN** 用户重复执行同一 CLI 传输命令而未显式提供幂等键
- **THEN** 确定性派生的键命中原 operation，恢复原传输而不创建重复文件

#### Scenario: Transfer progress is observable
- **WHEN** 上传或下载在 --events 下运行
- **THEN** stdout 流出 start→progress（bytes_transferred/bytes_total，节流）→end/error，seq 单调；人读模式进度只在 stderr 刷新

#### Scenario: Agent facts flatten nested payloads
- **WHEN** --agent 投影包含嵌套 map 或切片
- **THEN** 标量叶以点号键扁平化一层（fact.operation.status=…），切片以 _count 表达；多行与更深层留在 --json

#### Scenario: Index search survives the exclusive lock
- **WHEN** MCP 客户端持有进程锁时执行 find
- **THEN** 经只读快照按同语义过滤返回并以 read_only 标注；摘要明示命中仍需 stat 验证

#### Scenario: Event feed survives the exclusive lock
- **WHEN** MCP 客户端持有进程锁时执行 changes
- **THEN** 经只读快照按同游标语义返回（0=当前位置、ahead 拒绝）并以 read_only 标注

### Requirement: Soft delete and restore
软删除 SHALL 限制在空间内且可恢复：trash 移入空间内隔离目录并保留引用可查；restore 移回原位且同名冲突加时间戳后缀不覆盖；永久删除不进入 Agent 面。

#### Scenario: Trash keeps the reference queryable
- **WHEN** agent 软删除一个引用
- **THEN** stat 返回 trashed 状态；download 与参考交接以固定码拒绝；配额仍如实占用

#### Scenario: Restore never overwrites
- **WHEN** 恢复时原位已存在同名文件
- **THEN** 恢复为带时间戳后缀的名称，双方文件都保留

#### Scenario: Permanent deletion stays with the owner
- **WHEN** 通过 MCP/Agent 请求永久删除
- **THEN** 拒绝并指向用户专属入口（网页回收站或带显式 owner 确认旗标的 CLI）

### Requirement: Recent operations inbox
generic 桥 SHALL 提供只读收件箱工具：列出本机状态库近 30 天内更新过的完成或失败操作，按更新时间倒序且有上限；不写任何状态、不触发传输或恢复。CLI 的 `recent`/`operation` 检查命令 SHALL 在排他进程锁被持有时回退只读快照，并在投影中标明 `read_only`。

#### Scenario: Bounded recent window
- **WHEN** 调用 inbox
- **THEN** 仅返回近 30 天更新的操作，按更新时间倒序，最多 200 条

#### Scenario: Honest failure
- **WHEN** 本机状态库不可读
- **THEN** 返回 state_unavailable，不以空列表冒充成功

#### Scenario: Inspection survives the exclusive lock
- **WHEN** 已注册的 MCP 客户端持有进程锁时执行 recent 或 operation
- **THEN** 经只读快照成功返回（data.read_only=true）；需要 provider 的命令以类型化 state_locked 失败并给出可行动提示

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

### Requirement: Agent discovery surface
discovery SHALL 提供空间内检索、游标增量与内容寻址登记：检索命中后经 stat 校验再消费；游标只前进而首次订阅不回放历史；内容去重登记新引用而不覆盖远端对象。

#### Scenario: Search with stale index honesty
- **WHEN** 检索命中本地索引但远端已变化
- **THEN** 消费前 stat 校验失败即报告版本变化；索引未命中不宣称远端不存在

#### Scenario: Cursor subscription
- **WHEN** agent 以游标轮询空间变化
- **THEN** 返回新增引用、版本变化与操作完成事件；游标单调且不回放首次订阅前的历史

#### Scenario: Content-addressed upload
- **WHEN** 上传内容的空间内同 SHA-256 已存在
- **THEN** 登记新引用复用既有对象（或显式链接引用）而不重传字节，不覆盖远端文件

### Requirement: Machine-mode failure contract
CLI SHALL 在每种输出模式下为命令失败渲染完整投影：`--json` 输出携带真实 `error.code` 与 `message` 的单一失败信封，`--events` 输出 start（含 spec_version）+error 事件（seq 单调），`--agent` 输出必行加 `error.code`，人读模式在 stderr 给出一行失败说明与恰好一条恢复建议；第三方原始错误文本 SHALL 被折叠为稳定码，绝不进入 stdout/stderr。

#### Scenario: Machine mode failure is a valid envelope
- **WHEN** 任一命令在 --json/--agent/--events 下失败
- **THEN** stdout 收到一个合法失败投影，error.code 为真实类型码而非占位符，诊断文本只在 stderr

#### Scenario: Third-party text never crosses the boundary
- **WHEN** 底层库返回非类型码错误
- **THEN** 输出使用兜底码 command_failed，原始文本不出现在任何流

#### Scenario: Transfer terminal states are normalized
- **WHEN** 传输以 interrupted/cancelled/failed 结束
- **THEN** 信封 status 归一为 failed 并携带状态派生码（resume_required/operation_cancelled 等），操作明细保留在 data

### Requirement: Error code registry
系统 SHALL 维护单一错误码注册表：每个码携带英文恢复提示与中文文档条目；人读失败输出与 `docs/error-codes.md` 由该表驱动，并有同步测试防止漂移。

#### Scenario: Hint and doc stay in sync
- **WHEN** 注册表新增或修改条目
- **THEN** 同步测试要求 docs/error-codes.md 的表格同步更新

