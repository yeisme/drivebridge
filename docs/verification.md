# 本地验证记录

本轮实现为 pre-1.0 本地候选；没有真实账号操作、付费生成、部署或发布。

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
