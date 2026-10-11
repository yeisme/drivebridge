# 新版工作台验收与交接矩阵

本文件是待执行 test spec，不是通过报告。设计输入核对见 [reference](personal-workspace-reference.md)；当前实施状态见 [前端 tasks](../../../openspec/changes/drivebridge-personal-workspace-rebuild-v1/tasks.md) 和 [后端 tasks](../../../openspec/changes/drivebridge-personal-workspace-contracts-v1/tasks.md)。每项关联 PRD 能力 Rxx，不能靠截图替代接口/权限测试。

## 1. 统一测试入口

所有下列命令从 `cli/drivebridge` 执行。U：客户端既有 Vitest（`python3 scripts/check.py -- pnpm --dir web exec vitest run`，可按文件定向），验证 UI 状态、纯逻辑和网络适配；B：后端普通 Go tests（`python3 scripts/check.py -- go -C server test ./internal/service ./sdk`）；E：`python3 scripts/check.py -- pnpm --dir web exec playwright test`，使用现有 Playwright+Server/OIDC/S3 fixture 覆盖用户完整路径；V：同一 E 入口附安全截图和 DOM 测量。没有增加平行测试框架；直接执行底层工具避免嵌套模块 runner。FRONTEND 单端用生成 fixture/网络替身，不以 E 真实联调作为其退出条件；E 归 INTEGRATION 任务。页面布局可做 mock 浏览器检查，但必须标 component 而非真实业务通过。技术决定 D01–D14 见 [选型文档](../../architecture/personal-workspace-technical-decisions.md)。

新测试文件随实现创建并登记真实命令，执行前确认有测试实际命中。测试层分别写 unit/integration/component/e2e；不把 mocked UI、原型状态测试或 strict 结果说成真实业务 e2e。

| ID / PRD | Given → When → Then | 层级 / 必要证据 | 任务 |
| --- | --- | --- | --- |
| AC01 / R01 | 已记录旧行为与 dirty 文件 → 拆除旧个人入口 → 无误删未提交工作，新入口无旧 main/Finder 依赖，保护能力均有迁移去向 | U+构建依赖核对；基线/差异表 | F01–F03 |
| AC02 / R01 | 320/390/768/1024/1440/1920 viewport → 导航、工具条和详情切换 → 无页面横向溢出、焦点可回、选中工具条位移≤1px | V；浅深色固定截图和尺寸 | F03/F17 |
| AC03 / R01 | 键盘或触屏 → 搜索、菜单、分隔条、确认 → 操作不依赖 hover，44px 触控目标，Escape/Tab/箭头一致 | U+V；含 200% 缩放/reduced-motion | F03/F17 |
| AC04 / R02 | 两用户/两个空间文件 → 登录、目录查询、换空间且旧响应晚到 → 无名称/数量/正文串线 | U+B+E；隔离与取消日志 | F04/B03 |
| AC05 / R02 | 上传中断或创建响应丢失 → 刷新并重选 → 原键 lookup、原 operation、已耐久分片保留，无重复文件 | B+E；original ID 和 durable bytes | F05/B07 |
| AC06 / R02 | 打开旧版本后产生新版本 → 预览/复制/下载/比较 → 仍使用旧 ref；恢复创建新版本而非覆盖历史 | U+B+E；版本摘要与 owner 回读 | F04/F06 |
| AC07 / R02 | PDF/图片/视频/Markdown/CSV/XLSX/恶意 HTML-SVG → 打开与切换 → 支持格式真实读取；不执行脚本、公式或外链；不支持可下载 | U+E；版本、内存与解析预算 | F06 |
| AC08 / R03 | 编辑 Markdown 与属性时服务端版本变化 → 保存 → 409 保留草稿；重读比较；结构化项目资产不能绕过 owner | B+U+E；原/新版本与安全错误 | F07/B09 |
| AC09 / R04 | Skill/模板/包已保存但未检查 → 打开/选用 → 明确未检查，无安装/执行副作用；版本变化后旧检查失效 | B+U+E；无执行调用断言 | F08/B10 |
| AC10 / R05 | 收藏/集合跨连接含失权成员 → 保存/刷新/移除 → 绑定持久、不扩权、不删除原文件；快照固定 | B+U+E；集合 revision/引用 | F09/B09 |
| AC11 / R06 | 内容索引部分覆盖/无配置 → 搜索 → exact/top_k、coverage/observed_at 明确，不自动分析/付费；游标过期能重查 | B+U+E；查询次数/无 process 调用 | F09/B03 |
| AC12 / R07 | 无 Agent → 接入仅目录 A 的 read grant → 默认长期、网盘可独立用，token 仅一次显示，保存不自动探测/操作 | B+U+E；凭据阶段截图/trace关闭 | F10/B04 |
| AC13 / R07 | 两 Agent 同用户不同 prefix → 用旧/new HTTP、SDK、MCP 调用越界/篡改 actor → 全部拒绝且无资源名泄漏 | B+契约集成；各入口负例清单 | B05/B12 |
| AC14 / R07 | grant 过期/撤销与 complete 并发 → worker 试图发布 → 未发布部分受阻、已完成回执保留；失 secret 回原 grant 再显式签发 | B+E；ordinary concurrency tests，无 race | B04/B05/F10 |
| AC15 / R08 | 计划预览后源/策略/选择变化 → 确认执行 → stale、不可执行旧计划；重建新预览与摘要后才确认 | B+U+E；revision/digest/逐项结果 | F11/B06 |
| AC16 / R08 | 任务部分成功或结果未知 → 恢复/取消 → 原 operation/plan 不变、不重做成功项、不把取消当回滚 | B+U+E；原 ID 和次数断言 | F11/B06 |
| AC17 / R08 | Agent 回传产物 → 完成上传、接收、归档、重复点击 → staging 不可越界，接收不等于归档，归档幂等且冲突不覆盖 | B+E；两 Agent、跨用户、固定 version | F12/B07/B08 |
| AC18 / R09 | 只读或断连 storage → 浏览/检查/跨盘计划 → 已配置≠已验证；复制保留原文件，源清理独立确认 | B+U+E；真实云另验 | F13 |
| AC19 / R10 | 固定分享或旧版本讨论 → 新版本/撤权/缺成员 → 分享不悄悄更新，缺项禁发布；撤销不宣称召回副本 | B+E；share/manifest/thread revision | F14 |
| AC20 / R11 | Web 无原生宿主或离线/空间不足 → 打开设备/备份 → 显示真实不支持/阻断；无假后台任务、无假设备在线 | U+E；宿主 capability fixture | F15/B11 |
| AC21 / R12 | 未配置内置助手 → 从上下文请求助手 → 显示未就绪，外部 Agent 路径仍可用；后续助手必须绑定原上下文 | U+B；S5 的 runtime 集成另验 | F18/B14 |
| AC22 / R13 | 旧 files/flows/overview/teams/governance/分享深链 → 新客户端访问 → 同义视图和 scope、原恢复身份不丢 | U+E；alias 全表、角色反例 | F02/F16 |
| AC23 / R13 | 本机 view/公开分享入口、logout、换用户 → 新资产 → 正确 entry/权限；无 token/秘密进入持久存储/证据 | U+E；本机 owner 兼容读回 | F16/F17 |
| AC24 / R07 | 新 grant → 默认 never 或逐个选择 duration 3600/86400/604800/2592000/7776000/31536000 → mode 必返，never 的 expires_at 必返 null，限时按服务端签发计时；非法组合拒绝；旧 /agents 默认7天/最多30天/响应原样，历史缺失不变永久 | B+U+E；可控时钟覆盖到期等号，持久回读与旧 golden；未执行 | B01.2/B01.3/B02/B04.1/F10.1 |
| AC25 / R07 | active/disabled → 禁用/启用、撤销、轮转、期限修改及并发旧 revision → CAS 单赢，轮转保身份期限且旧凭据立即失效；改期限从成功时刻重计时，验证/幂等重放不重置；expired/revoked 只能新建 grant | B+U+E；原 grant、generation、期限和 revision 脱敏对照；未执行 | B04.1/B04.2/B05.2/F10.2 |
| AC26 / R07/R08 | 创建/轮转响应丢失 → 原键 lookup/rotate lookup → 同 grant/代次且 secret 不重放，人类显式重新签发；MCP/API 配置和 checks 不派发外部任务，无演示执行入口；刷新进度回原 Operation/Plan | B+U+E；无副作用断言、原 ID/调用记录与文件验收独立；未执行 | B03.2/B04.2/B12.2/F10.2/F10.3/F19 |
| AC27 / R14 | 两用户两设备、含旧 Agent → 分组 CRUD/归组/置顶、刷新/重启、并发旧 revision、删组与归组竞争 → 本人组织持久回读，名称冲突与 CAS 可恢复，无跨租户泄漏；删组仅解绑保 pinned、grant 与文件权不变 | B+U+E；DB 回读/无 revoke 调用、410 游标、手机焦点；未执行 | B15.1/B15.2/F20/F23 |
| AC28 / R15 | Go DTO 修改、手改生成物或 nullable/fixture 漂移 → 执行统一生成与漂移门 → schema/TS/fixture 同源确定性生成且 drift 必失败；旧 HTTP/SDK/MCP 兼容 fixture 不变 | CONTRACT；B01 实现后登记实际生成/检查命令与非零命中负例；当前仅设计，未执行 | B01.2/B01.3/B12.1 |
| AC29 / R05/R07 | 两 Agent 向固定版本建议标签 → 人类子集采纳/拒绝、同revision并发、版本过期或丢响应 → 独立来源与幂等回查，保留已有人工标签；Agent不得自批/借旧write绕权，S2不等待收藏管理 | B+U+E；SQLite回读、事务/CAS、两用户/Agent；未执行 | B16.1/B16.2/F22/F23 |
| AC30 / R02/R07/R08 | 任务页活动/空闲/后台/身份切换、429或写超时 → 轮询与恢复 → 活动2秒/空闲30秒、后台暂停、无重叠、尊重Retry-After、401/403停止；旧响应不能串身份，写入先原键查询不新建 | U+E；假时钟/网络替身验证调度，真实原操作回读另验；未执行 | F02.1/F05/F10.3/F11.1/F19/F23 |

## 2. 需求到实施证据的追溯

这是设计追溯表，不是另一套状态或证据台账；状态只由两个 change 的 tasks.md 维护。spec 以 capability + 精确场景名定位：F=前端 rebuild change，B=后端 contracts change。运行后将对应 AC 的真实 evidence 通过任务 CLI 登记，不能用本表替代实现。各 AC 验证命令按第 1 节 U/B/E 执行；S5 有明确另验门。

| 需求 | 页面 | 公共服务合同 | spec 场景（capability / Scenario） | 实施任务 | 必需证据 |
| --- | --- | --- | --- | --- | --- |
| R01 | 全站壳/首页 | me/auth、workspace/home、旧 route 参数 | F personal-workspace-shell / 先清理再建立新默认入口、手机和窄屏、键盘调整详情 | F01–F03、F21.1 | AC01–03；保护清单、DOM/双端截图、登录恢复 |
| R02 | 文件/预览/详情/上传 | files/versions/content/operations/lookup/upload-batches | F personal-file-workspace / 预览期间版本变化、恶意或不支持的文件、创建响应丢失、大目录选择 | F04–F06 | AC04–07；固定 ref、耐久字节、刷新/HTTP+DB 回读 |
| R03 | 知识库/编辑器 | document/properties/property-definitions/links | F personal-file-workspace / 文档乐观冲突；B personal-workspace-assets / 过期正文保存、字段类型迁移 | F07.1/F07.2、B09.2/B09.3 | AC08；409 草稿与原版本不变、私密反链过滤 |
| R04 | 资源库 | resource-descriptors/checks、collection snapshots | F personal-file-workspace / 资源仅已存储；B personal-workspace-assets / 资源更新版本 | F08、B10 | AC09；固定引用与只读检查、无安装执行 |
| R05 | 集合/标签属性/收藏 | favorites、labels/suggestions、collections/snapshots | F personal-file-workspace / 集合或链接成员失权；B personal-agent-grants / 多 Agent 对同一文件建议标签；B personal-workspace-assets / 目标撤权 | F09.1、B09.1/B09.2；S2建议 B16/F22/F23 | AC10/AC29；revision、来源隔离、人工CAS、删除绑定不删文件 |
| R06 | 搜索浮层/全页 | file-queries/content-queries/saved-searches | F personal-file-workspace / 内容能力未配置、部分索引返回候选、搜索稳定结果与失败 | F09.2、B03.1 | AC11；稳定/空/失败/覆盖不足、无 process 调用 |
| R07 | Agent 接入/卡片/详情/调用 | agent-grants/lookup/rotate/rotate lookup/checks、agent-invocations | B personal-agent-grants / 默认长期、六档限时及服务端计时、期限修改与验证、轮转丢响应与期限保持、终态重新接入、跨 Agent staging；F personal-agent-experience / 期限修改和冲突、外部接入而非演示执行 | F10.1–F10.3、B04/B05、B03.2、B12.2、B16、F19/F22/F23 | AC12–14、AC24–26、AC29–30；时钟/CAS/旧兼容/全入口反例及安全回读 |
| R08 | 任务/Agent 进度/收件 | workspace/tasks、原 plans、inbox-submissions/decision/archive | F personal-agent-experience / 修改上下文后确认、归档冲突或重复点击；B personal-workspace-projections / 原任务结果未知；B personal-workspace-assets / 归档未知或冲突 | F11/F12/F19/F23、B06/B07/B08/B12.2 | AC15–17、AC30；原 Operation/Plan、固定版本、幂等和目标冲突 |
| R09 | 存储/索引/用量 | cloud/readiness/usage/transfer-plans | F personal-file-workspace / 跨存储部分失败；B personal-workspace-projections / 聚合部分不可用 | F13.1/F13.2、B11 | AC18；连接观察与配置分离、复制源保留 |
| R10 | 分享/版本讨论 | delivery/manifest/receipt/thread | F personal-file-workspace / 分享与撤销 | F14 | AC19；固定清单、缺项禁止、旧分享深链 |
| R11 | 设备离线/自动化 | readiness/actions、automation-rules/intent、宿主公开合同 | F personal-file-workspace / 纯 Web 系统能力缺失；F personal-workspace-shell / 单个切片完成 | F15/F18.2、B11/B14.2 | AC20；Web capability fixture 与 S5 原生实测分开 |
| R12 | 上下文/助手入口 | 原 plans/refs、待就绪 runtime handoff | F personal-agent-experience / 内置助手未接入、跨端继续 | F18.1、B14.1 | AC21；无模型副作用，S5 runtime 实测后启用 |
| R13 | 设置/兼容/回收/能力 | 旧 Key/team/governance/local view/share、trash/retention/readiness | F personal-workspace-shell / 旧流程深链、切身份晚响应；F personal-file-workspace / 纯 Web 系统能力缺失 | F02/F16/F21.2、B11/B12.1 | AC22–23；完整 alias/角色矩阵、恢复身份与秘密隔离 |
| R14 | Agent 分组与置顶 | agent-groups/agent-organizations | B personal-agent-grants / 跨设备回读、删除分组、组织冲突和列表恢复；F personal-agent-experience / 组织刷新与冲突、删除分组与焦点 | B15.1/B15.2、F20/F23 | AC27；重启回读、两设备/用户、删组不改授权 |
| R15 | 全新增 UI/API | Go DTO→machine schema/TS/fixtures 单一生成入口 | B personal-workspace-projections / 前后端独立消费、生成合同漂移 | B01.1–B01.3、B12.1 | AC28；可复现生成与 drift 负例、旧兼容 golden |

## 3. 分期任务与退出条件

F/B 编号顺序不代表执行顺序。每个任务只声明 CONTRACT、FRONTEND、BACKEND、INTEGRATION 之一；独立前/后端完成时另一侧为 INTEGRATION_PENDING，仅 INTEGRATION 任务产生跨模块联调证据。同一路径不能并行写入；表中可并行只是依赖许可，不是子 Agent 授权。

| 阶段 | 任务范围与依赖 | 独立退出条件 |
| --- | --- | --- |
| D0 | 本轮 PRD/UI/API/数据/spec/任务与 D01–D14 技术决定；最新快照固定 | v39资源和观察证据、R01–R15全追溯、期限/组织/调用一致、任务无缺失/环；用户已解除历史v36前置 |
| S0 | F01–F03；B01.1–B01.3、B02.1/B02.2；B01 可与只读保护清单并行，字段消费者等生成合同 | 登录/CSRF、恢复身份、旧深链与兼容保留入口可用；schema/TS/fixture 生成和 drift 通过；旧数据升级/回滚保护不丢身份 |
| S1 | F04–F06、F13.1，复用已有文件/上传 API | 本地 HTTP+DB 上传、刷新恢复、固定版本预览、基础连接读回；上传发送完成不当作发布完成 |
| S2 | B03–B08、B15、B16；F10–F12/F20/F22；最小 B12.2→F19，完整门 F23 | SQLite真实回读的期限/生命周期、三动作、分组置顶、标签建议人工CAS、计划、调用/进度及收件；两用户两Agent和未知恢复；F19不等B16或S3，F23不等S3 |
| S3 | B09.1–B09.3/B10；F07–F09/F21.1；类型冻结后 mock 与后端可分别验收 | 工作台、知识、资源、集合、属性、搜索持久化、版本冲突和权限过滤，搜索稳定状态齐全；单元 mock 不算真实闭环 |
| S4 | B11/B12.1/B13；F13.2–F17/F21.2；F17.1 在所有必需 Web 切片稳定后 | 分享、自动化、设备能力、兼容管理完整；所有 AC 的 Web 场景逐项通过，双端/旧深链/浅深/200%/键触及普通静态构建门通过 |
| S5 | B14.1/B14.2、F18.1/F18.2；依赖公开 runtime/宿主合同 | 内置助手与原生离线/后台能力各自真实实测后启用，不用 Web/mock/文档勾完；不阻塞前期已验切片 |

## 4. 视觉与控件证据

固定截图包括：个人首页、空/满文件列表、目录抽屉、长文件名、文件预览与四页签详情、单元格浮层、Markdown 冲突、Agent 空态/表单/撤销确认（不含 secret）、计划 stale/partial、收件归档冲突、连接失效、unsupported 设备、旧治理入口。

固定 viewport：390×844 和 1440×1000 为主对照；320/768/1024/1920 为布局边界。手机另测 390×480 模拟键盘高度，真实 iOS/Android 键盘和后台能力仍独立验收。可调宽详情记录 min/default/max、键盘调整和缩窄自动降级。

追加 Agent 卡片、长期/各档限时表单、改期限冲突、分组/置顶、删组、任务进度未知、搜索稳定结果/空结果/失败/覆盖不足。每个截图记录路由/viewport/theme/场景 ID，与当前原型和正式 UI Spec 对比。不得先删除行为断言再更新视觉快照；模拟数据变化应说明，不能以放宽 1px 或隐藏组件避开问题。

## 5. 规模与非功能目标

- UI 默认每页 50 行，挂载条数有上界；10,000 行目录/计划 fixture 用服务端分页，不一次性请求全目录或把所有页塞进 DOM。
- 1,000,000 文件、10,000,000 事件、10,000 项计划、100 GiB 文件仍引用原文件平台规模任务，未测就记未测；这次原型核对不满足规模门。
- 性能基线在相同 fixture/机器/浏览器/网络条件下记录 cold/warm；目标是常见按钮 100ms 内出现反馈，详情加载不重挂载阅读器，重型 parser/图表按需加载；不把网络耗时计成渲染响应承诺。
- 断线、429、401、403、409、410、503 和非 JSON 错误体都有明确动作；不靠无限重试恢复权限。
- 用户数据缓存按 instance+tenant+user+space 隔离；切身份清敏感数据。恢复身份/偏好与 secrets 分离。
- 一次 secret 屏幕不截图，不开启 trace/video；日志和 stdout/stderr 脱敏。文件内容、临时 URL 和 Provider 请求不进入证据。

## 6. 分层退出门

G0：设计与 OpenSpec 校验，当前文档任务范围；不声称实现完成。

G1：新壳 + mock 状态 + 安全阅读器/恢复逻辑的 focused tests；可预览但不是后端联调通过。

G2：真实本地 Server+SQLite+OIDC/S3 fixture 从登录入口贯通文件与 Agent 收件；fixture 写入需刷新/重启回读，不能只验证 HTTP 成功响应。B12.2 先验 HTTP/DB/SDK/MCP，F19 接浏览器；此门不等待知识/资源全量；S2 完整退出由 F23 验分组置顶、标签建议/采纳、生命周期、计划和原请求恢复。不要求每个切片双数据库同步通过；PostgreSQL adapter 保留，实际兼容/迁移/规模独立验收。

G3：两端完整路径、旧 URL/API/本机入口兼容、静态检查与构建通过；不重复全量门直到实现稳定。失败标注 introduced/pre-existing/concurrent/environmental/ambiguous，修复只落本任务责任。

G4：真实身份、网盘、模型、原生系统、规模和发布各自独立证据。未执行明确标记，不能用 G0–G3 勾完这些任务。

每次运行写入唯一 owner 根 `temp/integration-test-runs/<run-id>/` 六件套（summary.json、command.txt、stdout.log、stderr.log、env.json、artifacts/）；失败保留原退出码和原身份。最终交接按设计、源码、mock、本地业务、真实外部、发布六层列明，不合并成一个“全通过”。
