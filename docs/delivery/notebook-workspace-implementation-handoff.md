# DriveBridge 多笔记工作台实现交接

2026-10-04 · 面向正式 Web 开发

## 交接结论

当前 OpenDesign 原型已经达到“可以开始正式 Web 实现”的设计成熟度，但它仍是交互与视觉基线，不是生产 bundle。正式代码必须进入 `cli/drivebridge/web` 的 React/TypeScript 模块，继续消费 DriveBridge Server 和 Pinax owner 合同。

- 设计基线：[多笔记工作台正式实现规格](../web/ui/notebook-workspace-production-spec.md)
- 现有阅读合同：[知识阅读空间交互规格](../web/ui/knowledge-reading-spec.md)
- 原型入口：http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/index.html
- 动效与交互截图：http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/review/notebook-motion-v1/index.html
- Pinax 映射：http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/pinax-notebook-contract-mapping.md
- 合同真源：[DriveBridge Server 工作区合同](../server/personal-workspace-contracts.md)、[Pinax 人类阅读 API](../../../pinax/docs/interfaces/human-reading-api.md)、[Pinax 人类阅读写入 API](../../../pinax/docs/interfaces/human-reading-write.md)

## 已经固定的行为

- 网盘双侧栏、笔记目录树、正文和关联面板的响应式布局。
- 多标签页、固定/关闭/重新打开、双窗格和可调分隔线。
- 阅读、实时预览、源码三模式，共享 Markdown 解析和 anchor 映射。
- 命令面板 `Cmd/Ctrl+P`、快速打开 `Cmd/Ctrl+O`、全局搜索 `Cmd/Ctrl+K`。
- 标题/列表折叠、区块工具条、悬停预览、反向链接和大纲跟随。
- 120/180/220ms 动效令牌、reduced-motion、触控端 44px 触点。
- 私有视图状态不写入 Markdown、Pinax 或公开 API。

## 当前证据

上一轮原型回归已通过：

- 16 组布局与主题组合。
- 15 项笔记交互流程。
- 22 项原网盘回归流程。

本轮动效层 HTTP 浏览器复核：

- 桌面命令面板、输入框快捷键隔离、区块工具条、标题折叠、大纲跟随和标签循环通过。
- reduced-motion 下动画为 `none`、过渡为 `0s`。
- 390px 触控端无横向溢出，悬停工具条不会自动遮挡正文，折叠触点为 44px。
- 控制台无页面错误或失败请求。

这些证据是浏览器原型验证，不等于真实 Pinax vault、生产权限、Electron 实机或真实云连接已通过。

## 正式实现顺序

1. 在 `web/src/features/notebook/` 建立正式工作台壳和私有状态。
2. 将原型中的目录、标签页、窗格、焦点和阅读位置行为迁入正式 React 组件。
3. 建立共享 Markdown parser/anchor map/render tree，禁止阅读和实时预览各自维护替换规则。
4. 接入文件/版本/权限/固定引用合同，删除生产路径对演示数据的依赖。
5. 接入 Pinax reader；能力未提供时显示 unsupported，不读取真实 vault 旁路补齐。
6. 接入 writer proposal → diff → confirm → apply → receipt；保留冲突、撤权和未知结果。
7. 完成浏览器、Server/DB、Pinax 合同和 Electron 验收后，再进入发布门。

## 开发与验证命令

```bash
cd /home/yeshugen/workplace/yeisme-agent/cli/drivebridge/web
pnpm build
pnpm prototype:unit
pnpm prototype:test
```

后续正式功能应使用 `web` 现有 runner 和 `temp/integration-test-runs/<run-id>/` 证据目录。禁止用 OpenDesign raw 资源哈希、静态截图或 mock adapter 代替业务测试。

## 正式实施任务映射与当前切片

本次只推进多笔记工作台相关待办及必要依赖，不把两个 change 的全部未完成任务视为已授权扩大范围。

| 能力 | 既有任务 | 实施与验收边界 |
| --- | --- | --- |
| 笔记壳层、标签页、双窗格与私有偏好 | F03.3、F07.1、F07.2 | FRONTEND；复用既有身份壳层，新增正式 notebook 模块；不删除未迁完的旧入口 |
| 统一 Markdown、安全渲染与三模式 | F06.1、F07.2 | FRONTEND；普通文件与 Pinax 共用渲染能力，编辑权限仍由 owner 裁决 |
| 大纲、双链、关系与快速打开 | F06.2、F07.1、F09.2、D05 | FRONTEND；固定来源/revision，未提供索引或关系能力时显示 unsupported |
| 阅读进度与收藏 | D01、D04、F09.1 | CONTRACT/BACKEND 分段；复用 application service 与 Favorite 规则，浏览器私有偏好不能冒充已持久化阅读进度 |
| Pinax reader/writer 接入 | D01、D05；writer 对齐后续 owner 合同 | reader/writer 公开合同已存在不等于 DriveBridge 已接通；旧 D05 只读任务不能被新 writer UI 验收悄悄改为可写 |
| 真实跨端与桌面验收 | F17 相关路径、D09、D10 | INTEGRATION；浏览器替身测试不勾选真实链路或三平台门 |

当前 `web/src/features/notebook/` 的状态基础属于 FRONTEND 部分实现：

- `notebook-state.ts`：内存标签页、预览复用、脏草稿关闭保护、固定身份、双窗格位置与撤权清除。
- `notebook-preferences.ts`：actor/space/source 隔离的布局偏好；显式字段投影禁止持久化正文或凭据，损坏/不可用存储安全回退。
- 已接入 `WorkspaceApp` 普通 Markdown 文件预览的可选“笔记工作台”入口，保留旧预览。仅普通文件只读，不能据此宣称完整知识工作台或 Pinax 已实现；OpenSpec 整项保持未完成。
- `NotebookWorkspace`、`NotebookSplit`、`NotebookDirectory`、`NotebookProperties` 提供容器响应式侧栏、标签页、双窗格、导航、专注模式与阅读偏好。右栏关系能力仍明确 unsupported。
- `NotebookDocument` 使用 CodeMirror 保留模式切换期间的编辑器历史；生产文件适配入口不给写权限。共享 renderer 提供 GFM、安全外链/外图处理、同 AST 大纲、标题和列表折叠；渲染失败保留源码并可重试。
- 现有 `latest-request.ts` 可复用取消与晚响应隔离；组件仍须按完整身份和 context revision 建立请求边界。

前端留在当前 checkout，不切换已有 dirty 分支。已单独运行 INTEGRATION 最小切片：现有 Go disposable fixture、独立端口 31987、真实登录/上传/固定版本文件读取及浏览器交互；未访问真实云账号或 Pinax vault。后端新增实现前仍须确认隔离工作区与可丢弃数据位置。

截至本次检查点：notebook 模块 14 个测试文件、57 项用例通过，证据 `temp/integration-test-runs/20261004T145952Z-73fce31a/`；最新定向类型检查证据 `temp/integration-test-runs/20261004T145836Z-a32b12cd/`。容器响应式侧栏加入后的最小真实文件 Playwright 链通过，证据 `temp/integration-test-runs/20261004T144638Z-370aabb1/`；其后的列表折叠、专注、错误恢复和命令方向键仅有定向组件验证，尚未重跑浏览器。不是全量质量门证据。

仍需完成的主要能力：双链与反链 owner 适配、悬停预览及嵌入限制、区块引用工具条、公式/Mermaid 安全按需渲染、跨标签和重启的完整私有状态、application service 阅读进度、正式保存提案/冲突/unknown 恢复，以及完整浏览器矩阵和 Pinax/桌面验收。当前不得勾选 F06/F07/D05 等整项。Pinax writer 与旧只读任务存在阶段差异，应先明确对应合同任务后接入，不走普通文件保存旁路。

## Pinax 与阅读状态前置核对

当前依赖不是“缺少 Pinax reader API”：Pinax P01–P07 已提供公开 schema、七个 reader endpoint、`pkg/pinaxclient` 与受监督 stdin 会话。消费者生成源是其 transport catalog；正文读取返回分页 DocumentPage 树，不应调用 source-export 把全篇下载到 renderer 来代替该合同。

DriveBridge 尚缺的承接层：

- D01：现有 `web/scripts/generate-contract-types.mjs` 仅从 `server/cmd/drivebridge-contracts` 生成 personal-workspace profile。B01.2/B01.3 已完成；desktop profile、ReadingRef/ReadingState/IPC 类型与固定 Pinax schema 摘要尚未加入。应扩展同一入口，不另建 TS DTO 真源。
- D02/D03：受监督本机门面及受管范围保护仍未完成。浏览器不能绕过这些步骤直接取 Pinax bearer 或读取 vault；metadata 预检后须确认 scope、安装 guard，再新建正文会话。
- D04/B09.1：Server 当前有 `WorkspaceFavorite` GORM 模型，但它明确排除本机阅读状态；B09.1 的共享 application 规则仍未完成。不可在前端或 Server 私自创建第二套收藏/阅读进度真源，也不可把本机状态上传云端表。
- D05：在 D01 生成类型和门面稳定后，消费固定 source/object/revision、分页/outline cursor 与 owner-resolved links；Pinax AST 不应重新用普通 Markdown 标题规则生成另一套锚点。

以上是待实施依赖，不代表外部服务不可用，也不将任务改为已完成。下一实施顺序为 D01 合同 profile → 共享状态 application 与受控门面 → reader adapter → 明确 INTEGRATION 验收；Web 独立渲染任务可继续，不等待真实 vault 授权。

## 全站复现推进检查点（2026-10-04）

用户已将范围扩大为整套最新 Web 原型及 DriveBridge 本地真实业务闭环；本文件早期“仅笔记切片”是历史范围，不再限制后续全站任务。真实 Pinax、原生三平台、真实云账号和生产发布仍独立验收。

本次只完成参考冻结与文内引用切片，未完成全站复现：

- 经 OpenDesign MCP 读取入口，新增 `scripts/prototype_snapshot.py`，通过 owner runner 双次读取入口和直接 script/stylesheet 共 25 项，生成摘要清单。范围不包括递归 CSS 字体/动态导入或全站截图，不宣称完整离线镜像。
- 快照：`temp/integration-test-runs/20261004T170854Z-6d058d07/artifacts/prototype/snapshot-manifest.json`；资源只作为参考，不进入生产 bundle。
- 同一 Markdown AST 解析文内标题别名及段落末尾 `^block-id` 引用，定位仅限当前渲染窗格，不更改应用 hash，自动展开目标祖先。缺失/重复目标、跨笔记引用和嵌入保留未解析；代码和普通链接中的标记不转换。
- 笔记模块 15 文件 67 项测试通过：`temp/integration-test-runs/20261004T171310Z-e35a9577/`。构建及类型检查：`20261004T171127Z-924d0e8e/`，存在大 chunk 警告。
- 实际 disposable fixture 上传/读取/引用跳转及桌面、390px 交互链通过：`temp/integration-test-runs/20261004T171207Z-3ae19adc/`。截图在该目录 artifacts 中。这不是六视口全主题或全站视觉通过。
- 原型采集脚本的边界/去重/漂移/清单生成 4 项测试通过；证据 `20261004T171315Z-88685cd2/`。

下一批继续全局正式壳层及页面/合同覆盖；随后补齐 Agent/收件、信息工作台和普通文件正式保存。当前生产笔记入口仍只读，跨笔记关系、反链、公式/Mermaid、完整状态恢复和正式 writer 仍未交付，F07.2 不勾完成。

参考采集命令（从 owner 根目录）：

```bash
python3 scripts/check.py -- python3 scripts/prototype_snapshot.py --entry http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/index.html
python3 scripts/check.py -- python3 -m unittest discover -s scripts -p test_prototype_snapshot.py
```

## 正式导航、普通文档保存与 Pinax owner 增量（2026-10-05）

上文“生产只读”是早期检查点，以下更新只覆盖已验证的增量，不表示原 F06/F07 或整个全站计划完成。

- 正式导航采用184/56px、1280px自动断点，支持用户独立偏好与恢复自动；旧路由和业务页保留。知识库使用真实 file-query/stat，逐50文件页检查 Markdown、每批最多6个metadata请求；显示已检查/已加载范围，不冒称全文或全库检索。
- 普通文档编辑端点由 Server 明确 `--document-editing` 开启（默认关闭；fixture启用）。主窗格 capability gate→无损草稿→差异确认→base_version/CAS→原 upload/version 回执；空正文与2MiB UTF-8预算、旧版本、同键重放已验证。Pinax 不能走此接口。
- 正文草稿仅内存。发送前用 Web Locks 原子登记非秘密恢复键；不支持锁/存储时保持只读。跨标签不会覆盖另一待决键；失权卸载全部私有窗格；保存换版保留独立滚动位置，不能清除新版已有草稿。未知结果只查原 operation；完整merge/rebase UI仍未完成。
- Pinax owner 已加 editor-source（固定revision/64KiB/元数据隔离）及只读 tags/graph/daily定位。真实 Pinax服务+typed client 的读取→提案→确认保存→新版本→同回执已验；CRLF特殊frontmatter仍明确只读。Pinax现已补日记首建propose/apply及同会话原回执恢复；新增接口间同日并发与模板改路径防重已验，日记正文读写兼容、标签mutation、rich-document、跨会话恢复尚未完成。详细合同归 [Pinax reader](../../../pinax/docs/interfaces/human-reading-api.md) 与 [writer](../../../pinax/docs/interfaces/human-reading-write.md)。DriveBridge没有直连本机vault；D01–D10仍待实施验收。

证据位于 owner 的 `temp/integration-test-runs/`：

- DriveBridge 46文件198测试：`20261004T192533Z-be2fcb31`；类型/UI `20261004T192539Z-d46b63a6`；构建 `20261004T192558Z-94e6367d`（仍有大chunk警告）。
- 三条真实fixture浏览器链：`20261004T192628Z-ab0bdc49`。覆盖导航320–1920、上传/打开/保存/原版读取/原键重放、两个真实浏览器tab的版本冲突与草稿保留，截图仅合成数据，不是全站≤1%像素差异证据。
- 最新笔记89测试与UI类型检查：`20261005T053420Z-636f712d`、`20261005T053426Z-1e357cf2`。
- Server定向文档测试：`20261004T190147Z-5d3af8fd`；生成合同漂移/39schema：`20261004T192533Z-e93c98c7`、`20261004T192535Z-7db57236`。
- Pinax源码服务最新回归：其owner `20261005T073504Z-3021529`；导航服务：其owner `20261005T073506Z-3024058`。日记首建：其owner `20261005T074452Z-3521605`（7包/31顶层测试）；这些均是owner/component，不勾选DriveBridge INTEGRATION。

下一阶段保持原全站范围：补其余Pinax合同及受控本机门面/guard/共享阅读状态；复用Web UI构建Electron，单独验收真实链路与macOS/Windows/Linux。未访问个人vault、云账号，未部署/push/发布，未运行race。

## 明确禁止的实现捷径

- 不把编译后的 OpenDesign JS/CSS 直接复制进正式应用。
- 不把 Pinax mock adapter 当作真实读写成功。
- 不在客户端判断权限后直接执行文件或正文写入。
- 不用标题猜测跨来源同一笔记；必须使用 source/object/revision/anchor。
- 不用新幂等键重试未知写入；先回查原 operation/receipt。
- 不把私有布局状态写入 Markdown frontmatter、Pinax 标签或公开 API。
- 不通过增加全局动画库解决普通 UI 过渡。
