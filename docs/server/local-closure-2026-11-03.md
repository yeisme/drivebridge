# 本地闭环实现与验收记录

本轮依据用户确认的本地全栈闭环计划实施。任务状态和完整证据由 `openspec/changes/drivebridge-personal-workspace-rebuild-v1/tasks.md` 的 LC01–LC06 维护；本文说明行为与恢复边界，不代替验收任务。现有工作区改动基线保存在 `temp/closure-baselines/20261102T155535Z/`。当前仍不是完整项目验收或发布候选的最终批准。

## 双库与原操作恢复

`make test-backend` 顺序运行 SQLite 与独立 PostgreSQL 容器上的完整 service 测试集合，也可用 `make -j2 test-backend` 在相互独立的数据上并行验证。旧双库记录 `20261102T172732Z-4359bb1b`、`20261102T173506Z-e0db6cc9` 以及下文后续记录确实运行了全集，但最初只有 external HTTP fixtures 选择 PostgreSQL，部分 internal helper 仍开 SQLite；不能据此宣称全部内部路径已在 PostgreSQL 验证。最终复核已纠正该缺口，见末尾数据库执行边界。PostgreSQL 无法启动时不得回退 SQLite。诊断 runner 的 `--skip` 只用于定位，带排除项的运行不能作为全集通过证据。容器只使用随机 loopback 端口和可丢弃数据；每个 fixture 只清理自身命名空间。所有聚合命令遵循不运行 race 的要求。

PostgreSQL 时间解码固定 UTC；新不可变回执在生成时采用两库共同支持的微秒精度，防止首次返回与原 operation 回读不一致。文件投影恢复沿用原 outbox revision，不从数据库时间重算纳秒。上传分片的保留列名交由 GORM 引用，数据库查询失败明确报错，不能伪装成空分片或未找到原 operation。

包含 NUL 的 item tuple 只改变数据库表示，公开 item key 与指纹保持原值。旧 operation 幂等键使用独立的 `KeyEncodingVersion` 识别编码，不能按字符串前缀猜测旧键是否编码；普通键和旧 SQLite 原始键继续可查。旧 operation 分配恢复仍验证完整源与目标指纹，不能把另一计划的产物视为当前计划结果。

## 备份与元数据迁移

PostgreSQL 备份通过只读 repeatable-read 事务、GORM 列类型检查与流式复制，生成兼容现有恢复入口的 portable SQLite 文件。源数据库仍是实际 PostgreSQL。所有源表均复制，复制后检查数量；不支持的列类型、数据库查询失败和不能证明静止状态均拒绝。失败只清理本次新建文件，不覆盖既有备份。`instance.key` 始终通过独立密钥渠道恢复，不进入备份包。

元数据 JSONL 的 API 行形状额外携带版本化 `_storage` 字段，保留 API 隐藏的 owner、operation key、授权与已封装密文字段；迁移指纹也覆盖这些字段。旧的仅公开投影快照缺少必要字段时拒绝导入，需从原状态重新导出，不能补造 owner 或重建 operation。数据库物理快照的旧恢复路径独立保留。快照包含敏感恢复材料，应仅写入受控本地文件，不能放入普通日志或测试证据。

## 桌面本地候选

Pinax SDK、随包 binary 与 CI 下载统一为 v0.3.1。构建优先使用明确提供的 binary；不再自动使用邻仓未固定源码。显式 `DRIVEBRIDGE_PINAX_DIR` 只生成本地源码候选。构建脚本验证 binary 模块版本、owner schema 摘要与已生成合同，记录来源和摘要，不把本地候选标为已签名发行产物。

```sh
DRIVEBRIDGE_PINAX_BIN=/absolute/path/to/pinax node desktop/scripts/prepare-host.mjs
pnpm --dir desktop build
DRIVEBRIDGE_HOST_INTEGRATION=1 pnpm --dir desktop test
python3 scripts/check.py -- xvfb-run -a pnpm --dir web exec playwright test --config playwright.electron.config.ts desktop-reading.spec.ts
```

Linux 开发验证保持 Electron sandbox、contextIsolation 和 webSecurity 开启。原生“工作区 → 打开已有工作区”选择 state 后，只经私有 bootstrap 交给 Go owner；不接受 renderer 路径、通用文件系统或任意请求。每个窗口独立 session、host、锁和资源路由，相同 owner state 引用复用已有窗口。缺失或损坏状态不得自动建立替代状态。原生菜单提供显式创建／迁移；来源登记只建立本机 Pinax custody，不配置云存储或凭据。实际首次创建、登记、确认阅读与重启通过 `20261102T184037Z-fc992f5e`，旧布局迁移保留元数据通过 `20261102T184611Z-e2914ce9`。

所有异步正文与控制请求在返回前复检窗口、宿主及请求代际；休眠立即作废原宿主，并发送不含数据的失效信号清除正文。晚成功与晚错误都不能恢复旧会话的显示内容。来源列表从 owner 持久绑定读取，后续正文调用仍重新取得 guard lease；重启不能提升 writer scope。

实际 Electron 的绑定、搜索、固定正文、多 state 拒绝交叉 source、原窗口复用、单窗口关闭及休眠清除已有通过证据。退出全部进程后恢复原来源与正文通过 `20261102T174417Z-19d3ef59`。收藏与稍后读已通过真实双 state 和重启验证；图片附件经 Pinax asset 登记后已通过实际解码。阅读位置的固定 revision/锚点恢复已通过，完整受控保存仍未闭环，不对外宣称桌面整体完成。

## 仍需完成的验收

旧预览侧栏、目录树和布局测试入口已按实际 UI 修正，共享 fixture 污染通过逐 spec 独立后端消除。首次隔离全集共 129 个 spec，其中 128 个通过；唯一弹窗定位失败 `20261102T201201Z-e69257ef` 修复后对应六视口/浅深色/键盘用例通过。新增目录分页用例的初始请求计数曾误包含目录树和工作区初始化重读，失败记录保留，最终完整目录 spec 三例通过 `20261102T203039Z-474a374f`。不以修复后的定向成功改写旧聚合退出码，也不关闭尚未逐项核对的 F17/F19/F23 父任务。百万目录、千万事件与真实十五分钟窗口已完成，见下方记录。旧版本源码重建证据不能替代实际分发产物，v0.1.2 旧安装包仍待提供。

三平台实机、自然使用观察、移动客户端、S5、真实云账号、生产数据库、发布部署均保持独立门。本轮没有运行 race、提交、推送或发布。

## 本轮最终回归进展

规模门已完成：`20261102T180232Z-3519f360` 实际 SQLite、1,000,000 目录、10,000,000 积压、累计 10,090,001 事件；20 并发读取，真实窗口 900.0277 秒，90,000 次发布（实际 99.9969 events/s）。投影 wall-clock P95 为 0.027264 秒，读取 P95 为 2ms，347416 个读样本；重复事件为 0，代次 GEN_0 一致。总耗时 3904.153 秒。该结果是本机受控负载验收，不是生产容量承诺。

规模修复后的完整 service 双库再次通过：SQLite `20261102T185614Z-420aeab4`（454.116 秒），PostgreSQL `20261102T190357Z-fd88dde7`（404.554 秒）。合同检查 `20261102T190222Z-500169fc`、113 schema 正反例 `20261102T190225Z-70a92e88`、40 desktop DTO schema `20261102T190225Z-fd5f6ca2` 通过。

Web 回归发现并修复了真实的预览高度回归：恢复既有 92dvh 规范，避免正文塌缩及 PDF 翻页控件被遮挡；PDF、CSV/XLSX、图片、HTML/SVG 与固定版本布局通过 `20261102T185741Z-54f51030` 的对应用例，不能把该混合运行整体记为通过。资源触控目标恢复到 44px，知识库工具栏支持窄屏横向滚动，固定版本元数据在窄屏保留可访问说明及信息面板。旧测试入口按实际目录/信息侧栏、真实分享及历史快照选择流程更新；空状态验收使用独立可丢弃 OIDC 身份，避免其他场景数据污染。

最新真实 bundle 宿主测试 `20261102T191325Z-b92eb810` 为 36/36，无跳过；桌面类型检查 `20261102T191326Z-880476a8` 通过。Electron 全集 `20261102T191327Z-6d6e848b` 为三条阅读流程通过、一条编辑流程失败：运行时不宣告 `proposeEditorDraft` 可用。该失败保留为未完成门，不删除或跳过，也不把受控保存标成已实现。

## Pinax 公开恢复合同待决事项

DriveBridge 的 writer 接线需要 Pinax owner 提供可公开消费的、原批准上下文来源。固定 v0.3.1 的 `localsession.RecoveryLookupGrant` 要求原 `principal_digest`、`scope_digest`、`request_digest`、operation/capability/binding 身份；当前公开 ready 帧与 SDK 没有交付原 principal/scope 摘要。恢复 grant 只能通过受信宿主私有通道发出，存储的摘要本身不是新授权。

建议的最小 owner 变更是：由 Pinax 在原人工批准建立时提供版本化公开批准上下文或等价的 owner-issued 不透明恢复引用，并验证其绑定的原操作、主体、scope、vault 和 request。恢复接口只查询原回执，不隐式 apply；新会话撤权、错误 vault、过期授权和身份不匹配必须拒绝。需同时提供 SDK、导出 schema、owner 测试和匹配 binary；不能仅升级 DriveBridge 的 TypeScript 声明。

DriveBridge 后续只消费该公开合同，持久化恢复引用，不保存正文、旧会话 secret 或私有 grant 帧。`WriterApply` 的 SDK 内置重发需经单次 dispatch transport 截止，丢响应只查询原 operation。原提案人工确认、冲突保留内存草稿、撤权清除正文与固定版本回读继续由真实 Electron 流程验收。当前未修改 `cli/pinax`，也未读取其私有 ledger 或复制其身份摘要算法。该 owner 扩展仍待用户决定。

## 可运行入口与能力验收表

在 `cli/drivebridge` 目录运行以下命令。桌面窗口通过原生“工作区”菜单显式创建或打开 state，再登记并确认可丢弃的 Pinax 来源；不会自动迁移或替代异常 state。

```sh
./desktop/out/drivebridge-desktop-linux-x64/drivebridge-desktop
make dev
```

桌面是本地 Linux x64 未签名阅读候选。`make dev` 是明确的可丢弃 OIDC／S3 fixture 与 Web 开发入口，地址由启动输出给出，不是云端部署或真实账号服务。不要把本地候选、固定 v0.3.1 Pinax binary 或源码重建旧版本误认作历史 DriveBridge 实际分发包。

| 能力 | 当前验收边界 | 证据 |
| --- | --- | --- |
| Guard scope、空 scope 与越权拒绝 | Go 根模块全集通过；历史实际分发包降级拒绝独立未完 | `20261102T191122Z-94407d2b` |
| BlobGC、原操作、元数据、备份与两库 | external 与 internal helper 均按实际数据库选择，service 全集通过；SQLite 特有兼容、JSONL 原表集合与全表 portable backup 分开 | `20261102T203805Z-3a276f9d`、`20261102T203805Z-fc648ad5` |
| 百万目录／千万事件／15 分钟 | 完整目标通过，非校准规模 | `20261102T180232Z-3519f360` |
| CLI／MCP 远端客户端合同 | 真实本地进程上传、固定下载、幂等回放及撤权通过；对象存储为本地替身 | `20261102T193324Z-b0fff3db` |
| Web S0–S4 | 129 spec／136 fixture 运行结束：135 运行通过，团队 spec 因新增测试助手断言失败；修复后团队 3 例与用户管理复验通过。无 skip；完整聚合原退出码非零，F17/F19/F23 未关闭 | `20261102T205218Z-df14ec99` 至 `20261102T211421Z-6964a6ad`；修复复验 `20261102T211314Z-e8602ed1`、`20261102T211315Z-61e537e7` |
| 六视口、浅深色与阅读布局 | 320/390/768/1280/1440/1920，全站空状态与笔记布局通过 | `20261102T191809Z-b0cca9f5` |
| Electron 阅读与多 state | 真实 Electron／DriveBridge／Pinax，首次配置、搜索、固定正文、重启和两 state 通过 | `20261102T202917Z-2cecc33f` |
| Electron 200% 与键盘 | Chromium 原生 `setZoomFactor(2)`、深色、reduced-motion、键盘收藏及随后休眠清除通过 | `20261102T202917Z-2cecc33f` |
| 阅读附件 | registered raster 图片实际解码通过；PDF/音视频未接入此通道 | `20261102T202917Z-2cecc33f` |
| Electron 受控保存 | 未完成：公开批准上下文缺口；保留编辑测试失败，未宣告 writer 可用 | `20261102T191327Z-6d6e848b` |
| Desktop bundle、类型与 Linux 包 | 36/36（包括两个真实宿主测试）、类型、ASAR/资源摘要/fuses 通过；非三平台实机 | `20261102T202917Z-c53fd12c`、`20261102T202917Z-4ab9b8e6`、`20261102T202917Z-476b00aa` |
| 基础工程门 | `make check`、合同检查、UI 检查、两个 owner OpenSpec strict 通过；后续小修定向回归保留 | `20261102T192646Z-2b1e0c55`、`20261102T192734Z-ef7f0de6`、`20261102T194153Z-81a50d96`、`20261102T193305Z-48075b76`、`20261102T193305Z-8996f63d` |

`make e2e` 为浏览器服务复制独立构建目录，防止其他检查的 Vite 构建替换运行中资源。每个 spec 独立启动和回收后端；普通 fixture 开启属性编辑和分享接收，团队、治理、Demo、自动化用各自显式 fixture。Key 复制两个场景分别进入个人和团队 fixture，既有两个测试均执行。标签字典、保留锁、传输计划、属性编辑、迁移和归档故障执行启用/禁用或正常/故障两种分支。脚本固定本次矩阵开关，避免继承调用者的 Demo/故障环境；一个场景失败仍收集其余证据，最终保留首个非零退出码。治理配置同样消费本次 Web 副本。runner 十项测试 `20261102T202340Z-5b787cb1` 通过。证据只记录固定白名单 fixture 开关和 Web index 摘要，不采集凭据或任意环境变量。此前失败、错误命令和中断记录均保留，不覆盖成通过。

## 最终增量与候选产物

文件列表显式请求每页 50 项，上一页步长和页码共用同一常量；服务未收到 `limit` 时仍采用旧客户端的 200 项。非法、重复或越界参数拒绝。分页清空文件及文件夹选择，防止批操作带上上一页不可见的目录。105 个目录的三页前进/返回与原 cursor 已通过真实浏览器；本次修复只改变列表分页，不改变固定文件、版本或写入 operation。新增选择隔离组件测试与目录读取共七例通过 `20261102T202803Z-8bec2608`。

分页合同后的 SQLite 全 server 模块通过 `20261102T201611Z-4a4ec980`（service 456.075 秒），真实 PostgreSQL service 全集通过 `20261102T201608Z-b8c8d96f`（419.014 秒）。`make check` 完成，最后的文件夹选择修复再通过 Web 全集 197 文件／1360 测试 `20261102T203058Z-cdf65bc6`、含类型检查的桌面/Web 构建 `20261102T202804Z-bf107c50` 与 UI 检查 `20261102T203059Z-daefe233`。合同生成一致性、113 schema 与 40 desktop DTO 检查分别为 `20261102T202556Z-d2e59221`、`20261102T202559Z-403a5584`、`20261102T202600Z-316534ce`。

最新候选复用该 Web 构建，真实 bundle 宿主 36/36、无跳过 `20261102T202917Z-c53fd12c`，desktop 类型检查 `20261102T202917Z-4ab9b8e6`，真实 Electron 阅读/多 state/首次配置三条通过 `20261102T202917Z-2cecc33f`，Linux 包与资源/fuses 检查 `20261102T202917Z-476b00aa`。主入口仍为 `desktop/out/drivebridge-desktop-linux-x64/drivebridge-desktop`，不是签名发布包。已知 editor 全集失败仍保留在 `20261102T191327Z-6d6e848b`。

当前剩余事项按性质区分：

| 类别 | 未完成事项 | 后续验收入口 |
| --- | --- | --- |
| 功能缺失 | Pinax PDF/音视频附件；受控保存/原 operation 恢复 | LC04.2、LC04.3，真实 Electron 流程 |
| 跨 owner 合同 | Pinax 发出版本化批准上下文或不透明恢复引用，SDK/schema/binary 配套 | 上述最小合同提案；未获准修改 Pinax owner |
| 证据缺失 | 历史实际分发包降级拒绝；F17/F19/F23 全部原验收条款与证据逐条对应 | D03.5；原任务不可仅凭测试总数勾选 |
| 功能与工程收口 | F02.2 旧个人壳/CSS 依赖清理；F06.2 当前仍保留概览/版本/来源与引用/内容理解/讨论/活动六页签，尚未收敛为规格的详情/版本/活动/关联四页签 | 原重建任务账本，迁移必须保留全部旧能力；不能以现有六页签测试绿色关闭 |
| 独立交付门 | S5、移动端、三平台实机、自然使用观察、真实云和发布部署 | 原独立任务；本地测试不代替 |

## 数据库执行边界复核

新增 test-only `databaseTestConfig` 使 internal service helper 与 external HTTP fixture 使用相同的 `DRIVEBRIDGE_TEST_DATABASE` 选择。PostgreSQL 要求显式测试 DSN，每个可丢弃 state 使用摘要命名的隔离 schema，服务重开保留原 schema，测试结束再清理。配置错误、数据库不可达和清理失败均报错，不转成 SQLite。内部测试不能 import `fixtures`（该包依赖 service），因此只在 test-only helper 复用其 schema 隔离规则，不向生产代码加环境开关。

BlobGC／归档／分享接收与重启／OneDrive 替身／云 connector 替身／团队及治理并发／服务身份这些内部入口已接入选择器。`TestInternalPersistenceUsesSelectedDatabase` 断言实际 GORM dialect；批次 outbox 发布测试也复用同一数据库 helper。PostgreSQL 定向 `20261102T203707Z-0beb8336`、SQLite 定向 `20261102T203748Z-0cd67971` 通过。最终 `make -j2 test-backend` 退出 0：SQLite 完整 service `20261102T203805Z-3a276f9d`（455.457 秒）、PostgreSQL 完整 service `20261102T203805Z-fc648ad5`（418.312 秒），最终 service vet `20261102T204008Z-eb36ee09` 通过；LC02 已按这次补全后的证据关闭。

以下测试有意保留 SQLite，不能计作 PostgreSQL 业务持久化覆盖：managed vault 本地文件围栏、旧 NUL 原始键兼容、workspace 初始模型/历史迁移、子进程 SQLite migration，以及 portable backup 的 SQLite 恢复目标。PG 备份源仍是真实 PostgreSQL。普通双库集合默认跳过显式规模 opt-in、真实 COS 和仅供子进程调用的 helper；当前没有真实 COS 凭据，ffmpeg 可用。要求的 `TestFPTeamRuntimeScaleMillion` 已单独以 `DRIVEBRIDGE_FP_SCALE=million` 实际运行并通过，未拿普通集合的跳过替代；其他旧专项规模和 100GiB 流量测试不因此声称已通过。没有运行 race。

## 本次停止点与完整 E2E 结果

最后一次 `make e2e` 已结束：129 个 spec、136 次独立 fixture 运行，135 次运行通过；172 个测试通过、2 个测试失败、0 skipped。唯一失败 spec 为 `teams.spec.ts`，证据 `20261102T211112Z-8e10b034`。原因是本轮测试助手新增了“选择后立即改变控件值”的断言，而权限变更必须先人工确认，原控件保持旧值是正确行为。已撤回这条新增且不适用的助手断言；原测试中的人工确认、权限持久化和接管保护断言保留。团队三例完整复验 `20261102T211314Z-e8602ed1`、用户生命周期复验 `20261102T211315Z-61e537e7` 均通过。

选择器的实际修复保留：通过控件 `aria-controls` 定位所属列表，不再误点另一个正在关闭的 portal 选项。更早的分页/Key 筛选/用户筛选失败也保留，最终整套中的这些场景均通过。全部现有场景已有成功证据，但最后一次聚合命令仍退出非零；本轮没有再执行一遍得到修复后的单次全绿结果，LC06 不关闭。CLI/MCP 真实本地进程链通过 `20261102T205213Z-51adcdd9`，包复核 `20261102T205542Z-f7c4f401`；`git diff --check` 及最终 owner strict `20261102T211415Z-c242fa19` 通过。

这是一份可运行本地候选和明确未完项的交接，不是完整项目完成声明。后续仍需完成上表中的产品缺口及跨 owner／实际分发产物门，才能关闭整个计划。
