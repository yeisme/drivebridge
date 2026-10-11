# 本地验证记录

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


本轮实现为 pre-1.0 本地候选；没有真实账号操作、付费生成、部署或发布。

## 远程个人空间真机闭环（2026-09-16）

配合 backend-server/drivebridge 首个真实内网部署（key-only + Caddy 内部 CA + 真 COS），本机适配器以限定 Agent token（read,write、空间级、30 天）完成三场景真实冒烟：

- 文档报告：upload 报告 → 固定 `drivebridge://<instance>/file/.../version/...` 引用 → 检索命中。
- 开发产物：确定性 tar.gz 上传 → 内容派生幂等键重放复用原操作 → 下载字节一致。
- 创作素材：小 PNG 上传得固定引用（参考交接能力未启用，保持诚实关闭）。
- 指标：首次往返 1.6s（目标 10 分钟内）；脚本化后人工介入 0 次。
- 注册：`install --client claude-code`（user scope）与 `--client codex`（~/.codex/config.toml），注册的完整 argv 握手复核 14 工具可用；内部 CA 已在系统信任库，Go 适配器无需 SSL_CERT_FILE。
- 锐角记录：tar.gz 时间戳导致同键异纹 → `idempotency_conflict`（符合设计，内容派生幂等键规避）；既有远端路径 + 新幂等键 → `name_conflict`（不覆盖既有文件，符合设计）；下载目标目录需预先存在（`local_write_failed`）。
- 密钥与内网地址不落仓库；分发文件与 CA 证书在部署机私有目录。

- `temp/integration-test-runs/20260911T041805Z-5bc71cf5/`：Go/race、CLI、Graph/OAuth、路径与身份隔离、版本、取消、上传回执丢失恢复、MP4 校验、成品回存。
- `temp/integration-test-runs/20260911T041812Z-4cc6fa40/`：真实 stdio tools/list、能力资源 schema 一致性与默认关闭。
- `temp/integration-test-runs/20260911T041814Z-5d28ffe4/`：go vet。
- `temp/integration-test-runs/20260911T041815Z-6d2259ff/`：CGO_ENABLED=0 构建。

MCP schema 初次比较因 tools/list 的排序不同失败，调整为按工具名称比较完整定义后通过。测试未弱化字段/schema 的比较。失败证据保留。

Scaena 专项验证见对应 owner 文档。真实 Gateway、OneDrive 和火山区域队列有效期不在本地验证结果中。

新增私有 state/socket 目录检查后，旧测试临时目录权限过宽被正确拒绝；修正测试目录权限并新增拒绝公开 state 的断言后，最终测试全部通过。

## activation-readiness（2026-09-11）

- `temp/integration-test-runs/20260911T143824Z-89c33284/`：go test -race 全量（含 doctor 单测与 testscript 两态用例）。
- `temp/integration-test-runs/20260911T143838Z-15594600/`：go vet。
- `temp/integration-test-runs/20260911T143838Z-bb619b66/`：CGO_ENABLED=0 构建。

doctor 预检只读：不创建 state、不取进程锁、`mode=ro` 打开 state.sqlite；HTTP 探测只证明可达，授权一律 not probed。agent 模式补充标量 `fact.*` 输出（排序键，嵌套只在 `--json`）。真实账号冒烟按 [activation-local.md](activation-local.md) 由用户本人执行后另记。

## generic-backends（2026-09-11）

- `temp/integration-test-runs/20260911T183826Z-23b57415/`：go test -race 全量（含 generic provider alias 后端 fixture：往返/嵌套根/软链接三态/handoff 拒绝/后端校验）。
- `temp/integration-test-runs/20260911T183851Z-3b60848a/`：go vet。
- `temp/integration-test-runs/20260911T183858Z-9122efea/`：CGO_ENABLED=0 构建（二进制 ~57 MiB，10 个内嵌 rclone 后端）。
- `temp/integration-test-runs/20260911T183907Z-7ac80f65/`：真实 stdio MCP 冒烟（tools/list、能力资源、默认关闭）。

generic 空间验证离线完成于 alias 后端（本地目录钉根），代表任意 rclone 远端的注册表行为；真实 Google Drive/Dropbox/S3/SMB 等账号登录与传输由用户按 README 步骤单独验收，不以 alias fixture 宣称云账号链路可用。remember 现保留同版本的缓存摘要与已验证媒体探测（此前纯列表会清空 SHA-256，属既有缺陷修复）。

## cli-config（2026-09-12）

- `temp/integration-test-runs/20260912T055022Z-428b9422/`：go test -race 全量（含 internal/config 发现序/严格键/优先级单测与 testscript 配置全链：init 拒绝覆盖、show 来源、env/flag 压栈、未知键拒绝、配置驱动 space-add、space list）。
- `temp/integration-test-runs/20260912T055053Z-ce371648/`：go vet。
- `temp/integration-test-runs/20260912T055054Z-eb922992/`：CGO_ENABLED=0 构建。
- `temp/integration-test-runs/20260912T055104Z-6dd69275/`：真实 stdio MCP 冒烟（cobra 树下 tools/list/能力资源/默认关闭）。

cobra+ viper 重构后旧行为等价：旗标名不变、`space-add` 为隐藏别名、无配置文件时纯默认。`--enabled/--handoff`/secret 不可来自文件或 env（spec 固化）。

## agent-workspace（2026-09-12）

- `temp/integration-test-runs/20260912T133712Z-368e720a/`：go test -race 全量（含 TestWorkspaceDirectories：mkdir 幂等/目录上传/按路径 list/download 回读/逃逸与根 mkdir 拒绝）。
- `temp/integration-test-runs/20260912T133726Z-5faf006b/`：go vet。
- `temp/integration-test-runs/20260912T133728Z-29c038a5/`：CGO_ENABLED=0 构建。
- `temp/integration-test-runs/20260912T133731Z-dcbfc603/`：真实 stdio MCP 冒烟（新 mkdir/path/dir 工具面 schema 一致）。
- testscript workspace.txt：config 驱动 CLI 全链（mkdir→upload --dir→轮询 ls→重试幂等→逃逸拒绝→install generic 无 token）。

OneDrive 目录路径（Graph path 寻址/逐段 mkdir/graphPost）经 fake Graph 单测；真实 OneDrive 目录操作由用户实机验收。CLI 传输命令的 --enabled 门豁免=用户本人终端语义（spec 固化）。

## soft-delete + discovery 批次一（2026-09-12）

- `temp/integration-test-runs/20260912T141339Z-39c64e8c/`：go test -race 全量（软删除全生命周期：trash→trashed stat→下载拒绝→purge 两步门→引用消失→restore 回读；search 大小写/垃圾过滤；changes 订阅窗口/零游标不回放/cursor_ahead）。
- `temp/integration-test-runs/20260912T141414Z-dbac0587/`：go vet。
- `temp/integration-test-runs/20260912T141416Z-3f7c7102/`：CGO_ENABLED=0 构建。
- `temp/integration-test-runs/20260912T141419Z-7a469ac9/`：真实 stdio MCP 冒烟（新增 search/changes/trash/restore 工具 schema 一致）。
- testscript：find/changes/trash/purge 双旗标门/restore 全链；CLI 传输命令改为同步终态（排队即退会在进程退出时取消后台传输——竞态修复）。

OneDrive Move/Delete（Graph PATCH parentReference/DELETE）经类型化错误映射实现；真实 OneDrive 移动/删除由用户实机验收。AGENTS 边界改写：禁止永久删除（软删除+恢复限空间内）。

## personal-agent-loop-v1（2026-09-12）

- Go 全量/race：`temp/integration-test-runs/20260912T161936Z-1555c6da/`。
- 首用自动 remote 发现和配置默认目录补充回归：`temp/integration-test-runs/20260912T162303Z-54d8e786/`；setup 输出命令名检查：`temp/integration-test-runs/20260912T162538Z-6e845b3c/`。
- Go vet：`temp/integration-test-runs/20260912T162054Z-88fd0701/`。
- 三场景文件闭环：`temp/integration-test-runs/20260912T162206Z-8864d582/`，报告、代码压缩包、图片使用相同空间、上传/下载、来源、固定引用、重放和回收站恢复合同，含 recent 与恢复已完成操作。
- 真实 fixture 服务＋OIDC＋stdio MCP＋普通远程 CLI：`temp/integration-test-runs/20260912T162443Z-bdcb1864/`，配置由 CLI 写入，上传回执/来源、固定引用下载、检索、幂等、撤销、禁止远程回退本地状态均通过。
- Linux/macOS amd64/arm64 四目标 CGO_ENABLED=0 分发构建：`temp/integration-test-runs/20260912T162442Z-a1488efc/`。
- 分发包 SHA-256、内容校验与本机 Linux amd64 二进制 smoke：`temp/integration-test-runs/20260912T162650Z-c0b38b42/`。其余目标为交叉编译证据，不声称实机运行。
- OpenSpec 严格校验通过；原始失败证据 `20260912T160838Z-0f7ab935` 保留，初次场景暴露 stat 丢失来源，已在公共查询路径修复并回归。

增量合同包括可选 caller/task_ref、observed_at、setup/config set/recent/resume/cancel、检索过滤和远程引用替代参数。无存量删除或引用迁移，旧来源缺省请求保持兼容。回滚保留文件、回执、可选列和凭据。远程 mkdir/changes/purge 仍明确 unsupported；本机独占 state、generic/加密空间交接限制未被绕过。changes 未升级为可靠自动触发器。

分发文件在 temp/distribution，未发布、推送或部署；真实账号和市场验证没有执行。试用演示与指标方案见 market-validation.md，不将准备材料计为市场验证成绩。
