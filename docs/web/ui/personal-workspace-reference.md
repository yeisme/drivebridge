# 当前 OpenDesign 原型核对与实现差异

最新核对：2026-10-01（本地日期；UTC 2026-09-30T18:30Z）。用户已明确改用“执行时读取并固定摘要的最新快照”，不再等待历史 v36。本轮冻结入口为 v39；后续原型变化只记差异，不自动扩大范围。设计输入：[DriveBridge · 我的网盘与 Agent 工作台](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/index.html)。该地址为当前内网预览，不是产品生产地址。

## 当前冻结快照：v39

[资源清单](../../../temp/integration-test-runs/20260930T183020Z-c693eca9/artifacts/snapshot-manifest.json) 与 [页面核对](../../../temp/integration-test-runs/20260930T183020Z-c693eca9/artifacts/snapshot-review.json) 同属本次只读采集。入口及 12 个资源各读取两次确认摘要相同；浏览器全部通过已捕获字节响应，未从远端混入新版本，非快照请求被禁止。没有改远端数据或生成真实凭据。

| 资源 | bytes | SHA-256 |
| --- | --- | --- |
| index.html | 1442 | `60a1ce55fb276d6bc7d356a9d95dc504fe908dcabd832f538be80cb734a756ee` |
| personal-agents-v15.js?v=39 | 1951339 | `2a50c07b02941f583baa14e1282e9af9a12d6fc5a9aa4ada588972fed76b9bce` |
| agent-workspace-v36.css?v=39 | 23298 | `e998912744482d688d812472436d4327191f622f7ece2e2a1e310453d71f3cba` |

其他 10 项资源摘要与下方 v37 捕获相同。19 个观察点覆盖 14 个桌面导航页、初始页、Agent 接入表单、稳定搜索默认结果，以及手机初始/Agent 页；均未观察到 pageerror 或页面横向溢出。这不是所有视口或所有业务状态通过：320–1920px、深色、键触、200% 缩放、搜索失败/零结果/覆盖不足和真实 API 仍按正式矩阵待验。

相对 v37，Agent 页新增“全部展开/全部收起”，任务状态筛选明确说明按账户/空间记住浏览器偏好；搜索等待稳定后显示“最近访问与收藏”2 项，不能称为用户查询成功或完整索引覆盖。接入表单仍是长期默认，浏览器演示配置和“演示调用”文案仍仅参考。正式分组归属/置顶/授权/进度由服务端持有，折叠/状态筛选是隔离的非秘密偏好，secret 不进入持久化；详见 UI Spec。截图还显示桌面 Agent 页中“产物与收件”的浅色强调与当前 Agent 项并存；正式壳层必须区分待办强调和唯一当前页 aria-current，不能照抄为双选中。手机按单列纵向排列，任务/调用区需要滚动，不据首页截图宣称完整移动业务路径通过。

## v37 历史版本漂移与证据（当时 D0 未决，现不阻塞）

只读抓取于 UTC 2026-09-30T17:15Z；[完整资源与查询比较](../../../temp/integration-test-runs/20260930T171548Z-91a4dfbf/artifacts/asset-comparison.json)。实际入口包含 12 个资源（旧 v33 为 9 个）：

| 实际资源 | bytes | SHA-256 |
| --- | --- | --- |
| index.html | 1442 | `159e2721f5fbcf18bac93b94b574d97a6b8af0a01397eb947c904a3491e6b2e8` |
| personal-agents-v15.js?v=37 | 1932108 | `476273692f79c69acc96dcbdcccd89adc768feda8b408eb969b1ed830a2ef43a` |
| personal-agents-v15.css?v=34 | 11006 | `093187fd99a5206c1d6eecc6549603405332a4522a1ff777cd5d64c57a2db8e8` |
| pine-sidebar-v28.css?v=33 | 15219 | `3e3e80fdd3bb33bddc9f61b065cc16a948033e151556910c4b1b02fc71c2778b` |
| search-overlay-v34.css?v=35 | 17553 | `7470f63c2d8fcb33ae3ede0eee2ad703b48ceb55b282afbab5d4c1b462ba0070` |
| agent-cards-v35.css | 11503 | `a567ca01b53f7593645911294ddfb80b4b14a59b73ad729a80e3631ae412110b` |
| agent-workspace-v36.css?v=37 | 12443 | `5800ee74e130c59ddec9896cd0811f8d911ef2cc336eb8354680f6a1413c515c` |

同次请求 `personal-agents-v15.js?v=36` 及 `agent-workspace-v36.css?v=36` 返回与上表 v37 完全相同的 bytes/hash；查询参数是可变缓存标识，不是历史版本仓库。没有找到可独立验证的本地 v36 快照，不把这些字节写成“v36 已核实”。当时 D0 因此暂停；随后用户明确选择最新快照，现以本页 v39 摘要作为本轮基线，历史 v36 未核实的事实不改写。

[本次双端/Agent DOM 与截图](../../../temp/integration-test-runs/20260930T171704Z-940832d0/artifacts) 覆盖桌面/手机首页、Agent 空态和接入表单、搜索打开瞬态。首页无横向溢出，捕获页面无 pageerror；仅为模拟原型读取，不是产品验收。此前复用旧页面遍历脚本在“搜索”侧栏按钮超时，失败证据[保留](../../../temp/integration-test-runs/20260930T171412Z-8a2f0be1)；新版已换顶栏搜索，未因此宣称全页遍历通过。一次捕获中 Escape 未关闭接入弹层便打开搜索，不能作为焦点/弹层正确性通过证明，正式 UI 仍按 UI Spec 验收。

### 相对已保存 v33 的可证实变化（当前 v37，不冒充 v36 精确差异）

| 区域 | v33 记录 | 本次读取 | 正式设计决定 |
| --- | --- | --- | --- |
| 资源 | 9 个引用；JS hash 4451ad… | 12 个引用，新增搜索浮层、Agent 卡片和工作区 CSS；JS hash 476273… | 摘要+URL 记录定位；精确 v33→v36 未核实，仅保留历史缺口；当前采用最新冻结快照 |
| Agent 期限 | 接入默认 24 小时 | 表单默认“长期有效（手动撤销）” | 用户要求为准：never/duration、六档限时，服务端计时 |
| Agent 组织 | 旧列表/筛选 | 全部、已置顶、未分组、管理分组；独立卡片样式 | 服务端本人组织数据、CAS 与跨设备回读；不改授权 |
| 进度 | 任务中心独立 | Agent 页新增“任务进度 / 本次会话”区 | 原 Operation/Plan 持久投影，不继承会话内模拟状态 |
| 配置存储 | 原型示范配置 | 表单说明浏览器长期保存；调用与产物仍是会话演示，不发真实 Key | 正式授权与组织走服务端，凭据不持久化到浏览器 |
| 调用入口 | 演示行为 | 文案仍提演示调用 | 删除演示执行入口；外部 MCP/API 发起真实调用，平台仅观察/验收 |
| 搜索 | 侧栏按钮，捕获查询瞬态 | 顶栏搜索/Ctrl+K 与浮层，仍捕获查询瞬态 | 保留全页深链，补稳定 ready/empty/error/partial/stale 与焦点恢复 |

## v33 历史读取与证据（保留，不作当前版本）

通过 OpenDesign 的 project/entry 读取确认项目身份；批量文件读取返回 `Unexpected response type`，随后只读 HTTP 获取入口实际引用的 9 个资源，并用项目已有 Playwright 在隔离浏览器上下文查看。没有生成、写入或修改远端项目。

- 项目 ID：`drivebridge-personal-agent-workspace`。
- 入口 `index.html`：1233 bytes；SHA-256 `482b6c06bfe177e12baa8483635cbdbcc01811945c73019897cf0aa7f8dd0264`。
- 主资源 `assets/personal-agents-v15.js?v=33`：1865289 bytes；SHA-256 `4451ad077e12b70e3184675afdc7ec570205bd5a57e4b1e17cbe0663aac6afb8`。
- 最新侧栏 `assets/pine-sidebar-v28.css?v=33`：15219 bytes；SHA-256 `3e3e80fdd3bb33bddc9f61b065cc16a948033e151556910c4b1b02fc71c2778b`。
- [页面核对证据](../../../web/temp/integration-test-runs/20260930T083931Z-db8d7734)：入口/全部资源摘要、15 个桌面入口（含默认云盘）DOM、桌面 1440×1000 与手机 390×844 首页、部分页面截图。
- [详情核对证据](../../../web/temp/integration-test-runs/20260930T084130Z-429c295c)：Agent 接入表单、任务计划详情、PDF 示例预览。
- 浏览器首页未出现 pageerror、两个首页无页面横向溢出。仅证明所列模拟页面能渲染与打开；没有执行授权、真实文件读写、完整回归或原生设备验收。
- runner 的通用 environment/layer 字段不能升级本次证据：本次是远端模拟原型只读核对，不是正式产品 component/e2e 通过。具体范围以上述过程与 artifacts 为准。

页面仍显示“演示 · v9.2”，但真实加载资源已经多次更新。不得拿标签或旧测试计数推断当前远端版本已验收。

## v33 历史实际观察

| 区域 | 当前原型事实 | 正式设计处理 |
| --- | --- | --- |
| 产品身份 | 个人空间 · 多 Agent；没有默认团队切换 | 默认个人体验；旧团队/治理作为受权兼容入口 |
| 侧栏 | 工作台、Agent、管理；240px 展开、68px 轨道；可折叠分组 | 统一一套壳层，当前页和待办在折叠态仍可知 |
| 手机 | 首页、文件、Agent、任务、我的五入口 | 320–767px 使用底部导航与全宽详情 |
| Agent | 初始 0 Agent；接入表单：协议、目录、期限、读取/建议/收件动作 | 没有 Agent 也能用网盘；服务端逐动作授权 |
| 任务 | 原计划 r1、before/after、固定来源版本、调整与确认 | 使用原 plan/operation，不造客户端任务号代替 owner |
| 详情 | 可拖拽宽度、键盘调整、固定到工作区、加载/重试 | 可访问分隔条；手机全宽；字段状态规范化 |
| 文件预览 | 阅读工具条、上下文件、版本、来源列表与右侧详情 | 替换模拟正文，复用安全阅读器并固定版本 |
| 知识/资源 | 紧凑表、分页、属性/关系、Skill 和模板待检查 | 补持久合同；保存不等于检查、安装或执行 |
| 存储 | 云盘/OneDrive/S3/Google Drive；连接/索引/用量 | 显示源码支持、配置、权限和实际观察的差异 |
| 设备 | 页面明确写内存模拟，原生能力未实现 | 保留需求，未实现显示 unsupported，不显示假设备在线 |
| 搜索 | 本次捕获为“查询中”瞬态 | UI 规格需补稳定成功/空/失败/覆盖状态，不能把瞬态当完成验收 |

## 迁仓前源码基线与新版处置（历史事实）

以下 Client/Server HEAD 是迁仓前独立仓库的历史定位，不是当前两个 owner；当前唯一 Git owner 为 cli/drivebridge，本次读取 HEAD `73609bfca42093968f1eee0eb8188d3624e4e970`，大量迁仓/其他既有修改保留。表中 src 路径相对 web/，HTTP/service 路径相对 server/。

历史 Client HEAD：`9f007d08dcf46067ba91f02eca02e7510fbf55b4`，存在既有未提交修改。
历史 Server HEAD：`f0504fbf7cfa41799ac3f36176e6437c3182736c`，同样存在既有未提交修改。HEAD 只是定位，实际设计依据包含本次读取的工作树；不清理他人改动。

| 当前路径 | 读到的事实 | 新版处理 |
| --- | --- | --- |
| `src/main.tsx` | 登录、导航、数据加载、文件/Agent/传输/管理混在大入口 | 拆除旧页面编排，建立薄入口、app 壳与 feature 页面 |
| `src/style.css`、`finder*.css`、`workspace-layout.css` | 多轮布局/样式积累 | 建单一语义 token 来源；分批删除旧页面样式 |
| `src/api.ts`、`consumer-flows/client.ts` | cookie/CSRF、固定引用、原请求恢复 | 审查后保留与收敛，不重写协议 |
| `src/Media.tsx`、PDF/Spreadsheet/文本阅读器 | 真文件安全预览与已有测试 | 保留内核，替换外壳；防执行与版本隔离继续验证 |
| `src/info-workspace`、`src/consumer-flows` | 旧总览/四中心有源码和历史本地实现记录 | 迁移数据适配和行为到新页，旧 UI 编排退役 |
| `prototypes/personal-agent-workspace` | 本地 v9.2 原型与样本状态 | 仅参考和回归资产，禁止生产 import 模拟 model |
| Server `http.go`、`*_http.go` | 文件、连接、upload-batches、organization-plans、content、delivery、automation 已注册 | 优先复用；新增投影与窄授权由新后端包承接 |
| Server `Agent` / `createAgent` | scope 为 read/write/delete/import/export，默认 7 天；旧 list 为数组 | 不改旧枚举/默认与响应；新增个人工作台授权资源 |

旧原型历史记录保留在 [v9.2 交接](personal-agent-prototype.md)。新需求和清理范围以 [PRD](../product/personal-agent-workspace-prd.md) 与 [重建清单](personal-workspace-rebuild.md) 为准，不把历史“143/137”等测试结果复用为本次验证。
