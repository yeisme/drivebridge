# DriveBridge 统一开发入口

从 `cli/drivebridge` 开始工作。原 MCP、Server、Client 的工作区目录入口已清理，旧 cd 路径不再可用。这是一个 Git 仓库、一套 active OpenSpec 和一组开发命令；CLI/MCP、Server 和 Web 是仓内模块。工作台 PRD 与前端重建任务已迁入，功能实现状态保持原样。

本次用户明确要求统一开发管理，因此默认同一 checkout 运行 `make dev`，通过独立 loopback 端口和 disposable 数据隔离。仅在确需并行分支时使用本仓 worktree，不再为前后端维护不同仓库。

## 常用命令

```bash
cd cli/drivebridge
pnpm --dir web install --frozen-lockfile
make dev
```

打开 `http://127.0.0.1:5173`。命令启动本地可丢弃 Server/OIDC/S3 fixture 与 Vite 热更新，不连接真实网盘或模型。Ctrl+C 停止本次启动的进程并清理 fixture 数据。两个端口必须空闲，已有服务不会被终止。

自选端口：

```bash
python3 scripts/workspace.py dev --port 5179 --api-port 8789
```

```bash
make build
make test
make e2e
make check
make specs
```

- build：CLI、Server 的 pure-Go binary 到 `temp/build/`，Web 到 `web/dist/`。
- test：两 Go 模块的普通测试与 Web Vitest，禁止 race。
- e2e：本地 HTTP/MCP 实际闭环与现有 Playwright 浏览器路径；不代表真实云验收。
- check：稳定后普通测试、vet、TypeScript 和构建；日常改动优先下方定向命令。
- specs：只列统一 active change，不推进任务状态。

前后端任务编号包含 F/B 前缀时，使用同仓 `python3 scripts/openspec-tasks.py` 维护；它保留任务状态和证据。历史 `go run ./scripts/openspec-tasks` 只支持数字任务编号，不用于新工作台任务。OpenSpec 初始化/校验仍使用 `openspec` CLI。

单模块迭代：

```bash
python3 scripts/check.py -- go test ./cmd/drivebridge
python3 scripts/check.py -- go -C server test ./internal/service -run TestFPReadiness -count=1
python3 scripts/check.py -- pnpm --dir web exec vitest run src/api.test.ts
```

统一入口的检查证据写 `temp/integration-test-runs/<run-id>/`。根 runner 应直接包底层工具，例如 `python3 scripts/check.py -- pnpm --dir web exec playwright test`；不要再嵌套内部自带 runner 的 `pnpm test`/`pnpm test:e2e`。旧模块 runner 仍可用并保留各自历史目录；新集成任务优先从根 runner 执行。恢复身份、密钥、rclone 配置、文件数据和服务启动参数没有迁移。

## 可丢弃归档故障恢复验收

仅fixture命令支持 `--fail-archive-publish-once`，默认关闭且要求显式数字loopback listener。它让首次 `workspace.submission.archived` 审计插入在真实事务内失败一次，验证发布回滚及原operation恢复；不增加HTTP故障控制端点，不属于生产serve配置。测试期间禁止连接真实数据。

```bash
CGO_ENABLED=0 GOFLAGS= python3 scripts/check.py -- go -C server test ./internal/fixtures -run TestArchivePublicationFault -count=1
CGO_ENABLED=0 GOFLAGS= DRIVEBRIDGE_TEST_ARCHIVE_FAILURE=1 DRIVEBRIDGE_TEST_PORT=32987 python3 scripts/check.py -- pnpm --dir web exec playwright test tests/submission-inbox.spec.ts
```

浏览器测试使用当前 `web/dist`，前端变化后先执行 `pnpm --dir web build`。不设置 `DRIVEBRIDGE_TEST_ARCHIVE_FAILURE` 则运行普通丢响应路径；故障fixture每次进程只触发一次，请定向运行该验收，不能把一个已消费的故障进程复用于重复证明。trace/video/screenshot关闭，凭据不输出runner。

## 受控整理本地验收

人工整理管理新增独立开关 `drivebridge-server serve --automation-management`，默认关闭；启用不创建后台执行器，不修改旧规则API。只在明确部署决策后由服务端操作者为真实环境开启。本轮只在可丢弃fixture测试：

```bash
pnpm --dir web build
CGO_ENABLED=0 GOFLAGS= DRIVEBRIDGE_TEST_AUTOMATION_MANAGEMENT=1 DRIVEBRIDGE_TEST_PORT=33989 python3 scripts/check.py -- pnpm --dir web exec playwright test automation-management.spec.ts automation-tasks.spec.ts
```

页面打开只读；“配置与运行”取得能力后，新规则默认禁用。先确认保存，再明确启用/预览/确认一次运行。预览漂移拒绝；丢响应保留原恢复键，只查询原回执，不重建任务。现有规则只支持添加标签；不运行备份、删除、外发或模型。字节预算与有效期来自已绑定策略，不虚构默认值。

两条核心真实本地链：

```bash
CGO_ENABLED=0 GOFLAGS= DRIVEBRIDGE_TEST_PORT=33989 python3 scripts/check.py -- pnpm --dir web exec playwright test file-note-loop.spec.ts agent-inbox-loop.spec.ts knowledge-workspace.spec.ts submission-isolation.spec.ts
```

使用临时服务/数据库/文件/身份；禁止将该fixture命令指向真实账号。Web单元替身、真实本地链、云端及发布分别报告。当前原型为2026-10-07冻结的[全站基线](web/ui/workspace-visual-gap-baseline.md)，下文v39与2026-10-05盘点是历史截面，不代表当前未实现。

## 当前返工验证与复审入口

2026-10-08首次整体完成申请未通过独立审计；C110–V114现已补齐实现与本地验证，待独立复审。当前[全站差距矩阵](web/ui/workspace-visual-gap-baseline.md)含逐页/逐详情/弹层的原型依据、实际实现、有意差异、合同与截图；UI Spec §3.15及OpenSpec design记录返工。图集：`temp/integration-test-runs/20261008T140543Z-2c1dd519/artifacts/visual-comparison.html`。旧204初始观察含loading，不参与当前稳定态验收；当前独立空fixture204观察为`140335Z-8cebb779`，非空/对话框另有实际业务与布局证据。

```bash
NODE_OPTIONS=--no-experimental-webstorage python3 scripts/check.py -- pnpm --dir web exec vitest run
python3 scripts/check.py -- pnpm --dir web typecheck
python3 scripts/check.py -- pnpm --dir web exec vite build --logLevel warn
CGO_ENABLED=0 GOFLAGS= DRIVEBRIDGE_TEST_VISUAL_FULL=1 DRIVEBRIDGE_TEST_PORT=33989 python3 scripts/check.py -- pnpm --dir web exec playwright test workspace-baseline.spec.ts
CGO_ENABLED=0 GOFLAGS= DRIVEBRIDGE_TEST_PORT=33989 python3 scripts/check.py -- pnpm --dir web exec playwright test files-layout.spec.ts notebook-layout.spec.ts file-preview-layout.spec.ts information-layout.spec.ts agent-layout.spec.ts grant-connection.spec.ts task-inbox-layout.spec.ts management-layout.spec.ts workspace-navigation.spec.ts
```

实际多格式链与笔记闭环可运行：

```bash
CGO_ENABLED=0 GOFLAGS= DRIVEBRIDGE_TEST_PORT=33989 python3 scripts/check.py -- pnpm --dir web exec playwright test file-note-loop.spec.ts agent-inbox-loop.spec.ts submission-binary.spec.ts notebook-workspace.spec.ts
```

团队回归另启`DRIVEBRIDGE_TEST_TEAM_DEMO=1`，不得与个人Key测试混跑；账号生命周期测试需要干净fixture，其他测试给成员分配数据负责人会合法触发接管确认。属性/跨存储回归使用`DRIVEBRIDGE_TEST_PROPERTY_EDITING=1 DRIVEBRIDGE_TEST_TRANSFER_PLANS=1`。这些变量只控制本地fixture，不改变生产配置。

当前Web155文件1120测试（`20261008T140400Z-4ce0dd4a`）、type/UI（`140335Z-a30ed9d2`）、build（`140246Z-6c6cef92`）通过；Server普通测试全模块（`124821Z-ee3e4959`）及vet/build通过。6条当前真实本地浏览器场景（`140404Z-b6b40e0d`）覆盖文件/笔记原键恢复、Agent实际受限调用/验收归档、PNG/PDF/二进制原字节及撤权、普通笔记导航。基线必须独立空fixture运行，不能与写入测试共享服务后仍断言空态（混跑失败`134558Z-f15cfbc2`已归因并独立重验）。构建仍有>500kB chunk警告。9路径LSP检查6确认clean、3未确认，不能据此说全树LSP clean。未提交、推送、部署、执行race或接触真实云。后端隔离工作区`temp/automation-safety-worktree`留存本轮未提交成果；完成本地验证不授权强制移除该工作区。

## 目录和职责

| 目录 | 内容 |
| --- | --- |
| cmd、internal、go.mod | 既有 CLI/MCP 和本机能力 |
| server | HTTP、身份、GORM、存储/传输、SDK、fixture |
| web | React/TypeScript/Vite、阅读器、页面和浏览器测试 |
| docs | CLI 文档以及 server/web 模块文档 |
| openspec | 所有 active changes、base specs 和历史归档 |
| scripts/workspace.py | 开发、构建和检查编排，不持有领域状态 |

两个 Go module 名称保留，避免破坏外部 SDK import；一个 Git 管理它们，`make test` 会覆盖两者。不要把 `go test ./...` 误认为自动覆盖嵌套 Server module。

## 产品和实施

- [工作台 PRD](web/product/personal-agent-workspace-prd.md)
- [前端 UI 规格](web/ui/personal-workspace-ui-spec.md)
- [前端清理重建任务](../openspec/changes/drivebridge-personal-workspace-rebuild-v1/README.md)
- [后端工作台任务](../openspec/changes/drivebridge-personal-workspace-contracts-v1/README.md)
- [仓库迁移记录](repository-consolidation.md)

仓库合并本身不等于前端完成；本轮Web对齐与本地闭环已有上述分层证据，但整体审计仍未通过，不再从历史清理任务重建。网页与服务端可独立部署，但日常开发只维护本仓；部署与完整原生/云端验收仍单独授权。

## 本轮设计与分期任务

以下为最初设计检查点，后续实现状态以 [脚本生成的实现盘点](delivery/implementation-inventory.md)、各 owner 任务和证据为准，不把历史“未实现”解释成当前没有代码。

最初修订仅交付设计/spec/任务，不改业务实现。用户确认采用执行时冻结的最新原型，本轮为 v39，见 [版本与摘要](web/ui/personal-workspace-reference.md)；保留历史 v33/v37 与 v36 未核实记录，不再等待历史 v36。单机 SQLite 优先、PostgreSQL 保留兼容与独立实测门，详见 [技术选型](architecture/personal-workspace-technical-decisions.md)。

实施顺序为 S0 保护/共享合同/新壳 → S1 文件 → S2 Agent → S3 信息工作台 → S4 完整 Web；S5 助手/原生另验。详见 [需求追溯与分期退出门](web/ui/personal-workspace-acceptance.md)。首条 Agent 链用 B12.2/F19，不等待知识库/资源库；完整 S2 由 F23 汇合 B15/F20 的组织数据、B16/F22 的标签建议/人工采纳和计划生命周期；F19 不等待知识资源或新增建议链。B01.2/B01.3 固定 Go DTO→JSON Schema/OpenAPI3.1→TS、Go样例fixture及Ajv开发期校验。`make contracts`／`make contracts-check` 现已实现 personal-workspace 合同生成与漂移检查；D01已分片扩展同一入口的desktop profile（32个本机DTO与Pinax reader owner schema闭包）；完整路由/IPC仍未完成，不另建第二生成器。`make contracts`/`make contracts-check`覆盖两个profile，desktop生成需兄弟 `cli/pinax` 源码及Go工具链；单独旧profile可用 `node web/scripts/generate-contract-types.mjs --check --profile personal-workspace`，不加载Pinax。

F/B 任务只通过以下已存在工具修改正文或追加任务；amend 保留原状态和历史证据，append 只加新 ID，不用 init 覆盖现有任务：

```bash
python3 scripts/openspec-tasks.py --help
openspec validate drivebridge-personal-workspace-rebuild-v1 --strict --no-interactive
openspec validate drivebridge-personal-workspace-contracts-v1 --strict --no-interactive
git diff --check
```

实现任务按各切片独立验收，不能整体宣称完成；strict/链接检查仍仅为设计证据。`web/scripts/check.py` 当前默认 Vitest、不含 race；F01.2 核验其回归保护，不重复删除 race。历史设计轮未运行产品测试。后续本地实现使用定向真实 fixture，仍禁止真实云、部署及 race。

## 全站改造续接盘点（2026-10-05）

[实现盘点](delivery/implementation-inventory.md)由源码扫描脚本生成，列出页面/当前入口/组件、工作区合同/字面路由注册/Web消费线索、任务计数及测试发现。它不是运行时 capability：生成类型不等于前端使用，声明路由不等于真实授权或业务验收。

```bash
python3 scripts/implementation_inventory.py --write --evidence-run 20261005T082352Z-1aa2a4b3
python3 scripts/implementation_inventory.py --check --evidence-run 20261005T082352Z-1aa2a4b3
python3 scripts/check.py -- python3 -m unittest discover -s scripts -p 'test_implementation_inventory.py'
```

该次服务检查在 `CGO_ENABLED=0` 下运行 `TestWorkspace|TestDocument`，日志有96个不同顶层PASS；静态发现97个，其中migration process helper在父用例之外跳过，不能重复算一次独立通过。SDK命中的是生成fixture合同测试，不是新增Agent端点typed client全链。新grant/group/submission/suggestion已有服务实现，但Web仅生成类型中出现这些路径，没有发现对应正式请求字面量，现有Agent/任务/收件仍为旧入口。`agent-invocations`在合同中声明，未发现对应字面HTTP注册；不能把它标成可用。

下一执行前沿：D01 desktop合同 → D03受管保护 → D02/D04门面与本机状态 → D05/D06/D07适配及Electron；writer消费另立任务，保留原只读验收。所有页面需求继续保留。

D03已发现的前置风险：当前根模块 `internal/bridge/model.go` 的 `New` 直接AutoMigrate并恢复operation，未发现既有state版本拒绝门。不能只新增guard字段便声称旧binary被阻止。须验证可执行的迁移/准入与旧进程保护方案；不能证明的旧state/overlay/未知writer必须拒绝绑定。该发现不是已实现guard，也不是整个目标的外部阻塞。

## 2026-10-09 定向实现与验证

- 属性迁移只支持最多 200 个固定版本 sidecar，明确选择转换策略；不修改 Markdown/YAML 或 provider 文件字节。执行成功时字典与全部值原子切换；存在不兼容或漂移时只记录逐项结果，不部分切换值。preview、confirm、execute 各自使用原幂等键回执恢复。服务定向测试通过：`temp/integration-test-runs/20261008T182236Z-167d2603/`。Web 组件 36 项通过：`20261008T182221Z-369bf167`；真实浏览器预览→确认→执行丢响应→原键恢复→字典/sidecar 回读→结束清指针通过：`20261008T183157Z-91afba62`；默认关闭只读场景通过：`20261008T183246Z-a2397cb6`。
- 人工标签字典及选定文件合并由 `serve --label-management` 显式启用，默认关闭。保留源/目标字典身份，不表示全局删除或重写。合并预览丢响应按原键查找；确认/执行丢响应读取原计划，不制造阶段回执。服务流程通过：`20261008T182449Z-32808097`；修正的跨文件锁顺序定向检查通过：`20261008T182858Z-df7a51f3`。SQLite 的锁顺序检查不代表 PostgreSQL 跨实例并发已实测。
- 桌面 unsigned Linux amd64 DEB 已实际生成：`desktop/out/make/deb/x64/drivebridge-desktop_0.1.0_amd64.deb`，打包及资源/ASAR/Fuse 检查证据 `20261008T181735Z-10f74cf1`。原生 sidecar 构建证据 `20261008T181628Z-0b63368b`；桌面 28 测试通过、2 跳过：`20261008T181533Z-42ec6a1b`。包内程序在 Xvfb 下使用 sandbox 启动并显示本机预检页面；仍明确拒绝未通过受管保护的正文绑定。这不等于安装/升级/卸载闭环、三平台实机验收或签名发布。
- 固定目标 handoff 恢复定向检查通过：`20261008T181708Z-e650af6f`。未知写入结果在恢复与取消期间保留，生产 basename 列表按原目录/目标匹配，不因恢复重新选择上传目标。
- Web 全量 168 文件、1242 测试通过：`20261008T184250Z-35913de3`；UI 原语/样式检查通过：`20261008T184239Z-2a994823`。标签草稿固定原始字典修订，重读不得自动升级 CAS；并发未编辑字段回归包含在 `20261008T184206Z-35b07129` 的 34 项中，独立审查已关闭该问题。
- 双 profile 合同无漂移，103 个工作区 schema 与 37 个 desktop schema 正负例通过：`20261008T183418Z-8b7d88a2`、`20261008T183421Z-37f0d5ca`、`20261008T183422Z-c0db8248`。全部 80 项 OpenSpec strict 通过；仅规格有效，不代替运行验收。
- 最终构建与真实标签/属性双流程通过：`20261008T184216Z-519b0964`、`20261008T184341Z-50851d5e`。标签字典丢写响应、选定文件合并丢执行响应均按原身份恢复；后端回读验证未选文件、其它 namespace 与字典身份不变。标签默认关闭场景通过：`20261008T184009Z-6091066d`。构建仍有既有大 chunk 提示，未以调整告警阈值掩盖。
- Server service/SDK 全量通过：`20261008T184039Z-d533eb97`（service 371.173 秒，SDK 2.505 秒）。首次全量 5 分钟总时限耗尽，不计通过；使用 12 分钟总时限完成，未跳过测试。pure-Go server 构建通过：`20261008T183553Z-39449a6e`，实际 `serve --help` 已显示默认关闭的 `--label-management`。
- 根模块 `go test ./... -count=1 -timeout=8m` 全量通过：`20261008T184447Z-291e2077`，包含 CLI、bridge、desktop、remote；没有运行 race。
- 百万文件组织规模验收实际尝试 `DRIVEBRIDGE_FP_SCALE=full`，但现有证据 runner 在 900 秒超时（exit 124）：`20261008T183609Z-b6456d37`。没有获得完整分页/万成员快照/P95 结果，`drivebridge-file-organization-v1` 5.2 保持未完成，不能据普通回归通过宣称达到规模目标。
- 规模续查发现旧 `idx_files_owner_updated` 实际只有 updated 列。SQLite `EXPLAIN QUERY PLAN` 显示 owner 过滤后两条查询均需临时排序；新增 `idx_files_catalogue_order(owner, updated, id)` 后，目录修订读取使用覆盖索引，默认分页使用有序索引，均无临时排序。只增加索引，不改变游标或授权合同；已有数据库通过 AutoMigrate 添加，旧索引保留以避免破坏性迁移。定向大小分页/集合回归通过；这项查询计划改善本身不代表百万规模验收通过。
- 后续 10k 超时栈定位到 `CaptureUsage`：每次 POST 查询之后按每个文件分别查询历史版本，导致大量数据库连接等待。改为按 owner/space 约束并连接匹配 owner 的 File/Version 聚合，一次计算 current/history/trash，不加载所有文件或逐文件查询。修复后 10k 完整分页、1000 成员集合/快照及 20 并发场景在 10.52 秒完成，页 P95=1.399951409 秒；用量审计隔离回归同时通过，证据 `20261008T190915Z-35d1c620`。这是本机 SQLite quick 数据，不是百万或 PostgreSQL 性能证明。
- 百万规模最终通过：`20261008T200220Z-5dd81cb3`，`DRIVEBRIDGE_FP_SCALE=full` 实际验证 100 万文件完整分页、10000 成员集合/快照与撤权隔离；20 并发页 P95=695.350818ms，总用时 359.82 秒。修复还包括给 timestamp keyset 增加等价索引边界、保留审计但不对只读 POST 查询重采容量、fixture 批量插入及按原 VersionID 查播种来源。日志中的 quick-mode 是既有文案，数据规模由 full 环境值决定。5.2 已按本机证据完成；不宣称 PostgreSQL 生产性能。服务全量在聚合修改后通过 `20261008T191004Z-90e1ae03`，最后查询修改由组织/分页定向 `20261008T200139Z-8654b313` 及上述百万真实 HTTP 场景覆盖。

本地复验命令：

```bash
CGO_ENABLED=0 go -C server test ./internal/service -run 'TestPropertyMigration|TestPropertySidecar|TestWorkspaceLabel' -count=1
pnpm --dir desktop exec vitest run
node desktop/scripts/prepare-host.mjs
pnpm --dir desktop build:host
pnpm --dir desktop make
```

2026-10-09 内网体验修正：桌面导航隐藏原生滚动条但保留 overflow-y:auto，使用现有 no-scrollbar 工具类与显式 scrollbarWidth 覆盖旧 finder 样式。已在真实登录页面 1280×600 验证 scrollbar-width:none 且 scrollTop 可变化；前端构建通过。空间配额属于部署数据，本次仅为指定普通账号配置 10 GiB，不改变产品默认配额或其他用户权限。
