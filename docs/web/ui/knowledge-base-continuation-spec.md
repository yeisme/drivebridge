# DriveBridge 知识库续接规格

2026-10-18 · 本文件把 OpenDesign 当前知识库原型的增量收敛为可实施的 Web/Server/桌面交接。它补充[知识阅读空间交互规格](knowledge-reading-spec.md)与[多笔记工作台正式实现规格](notebook-workspace-production-spec.md)，不把原型脚本、示例数据或静态截图当成生产合同。

## 1. 原型输入与证据边界

本轮只读读取 OpenDesign 项目 `drivebridge-personal-agent-workspace`：入口 `index.html`，当前入口把 `#/knowledge` 接到独立知识库模块；原型方案文件为 `kb-integration-plan.md`。OpenDesign MCP 的项目读取确认入口存在，但项目文件列表接口曾返回空，因此正文以入口、方案文件和搜索结果交叉核对；不能把空列表解释为原型没有知识库。

原型方案的关键约束是：复用 Precision Pine 变量；只有 `#/knowledge` 接管页面；`?kb=classic` 可回到原版；正文仍以网盘 Markdown 为唯一真源；页面树、块编辑、版本历史、离线草稿/并排合并、页面数据库和图片块都属于原型能力；多人实时协作、评论同步、服务端全文搜索、关系图、模板库、日历视图和固定版本图片嵌入仍是缺口。

原型读回地址（当前 daemon 会话有效）：

```text
http://127.0.0.1:7456/api/projects/drivebridge-personal-agent-workspace/raw/kb-integration-plan.md
```

这份地址和 OpenDesign 读回只证明设计输入。正式行为仍以 `cli/drivebridge/web`、`cli/drivebridge/server`、Pinax owner 合同和对应 `temp/integration-test-runs/<run-id>/` 证据为准。

## 2. 知识库能力账本

| 能力 | 正式 owner/合同 | 本轮规格口径 | 原型状态 |
| --- | --- | --- | --- |
| 来源范围与 Markdown 列表 | DriveBridge Web + Server file-query | 绑定 `actor/tenant/space/source`；分页、游标、扫描覆盖度和不可用项分开显示 | 已做，部分仍是原型数据 |
| 物理目录与新建 | DriveBridge Server `/browser`、`POST /directories`、`POST /notebook/pages` | 网盘物理目录是层级真源；笔记写入 `<directory_path>/<title>.md`；`parent_file_id` 仅兼容读取 | 已做本地闭环 |
| 阅读/实时/源码 | DriveBridge reader primitives | 共享 Markdown AST 与 anchor；源码块原样保留；安全失败可回源码 | 已做 |
| 保存与冲突 | DriveBridge document save + `base_version_id`/CAS | 草稿 → 差异 → 用户确认 → owner 保存 → receipt；409 保留双方内容 | 已做本机/fixture，生产回执仍独立验收 |
| 离线草稿与合并 | DriveBridge Web 私有草稿 + Server 版本 | 草稿只存本机；恢复先查原身份；三方合并后人工选择，不能换键覆盖 | 原型已做，服务端同步缺口保留 |
| 页面数据库 | DriveBridge properties/collections 合同 | `pages` 代码块只是视图声明；属性写入走 property owner，不复制正文 | 原型已做，审计/状态枚举需对齐 |
| 图片与附件 | DriveBridge file/version/media 合同 | 图片按固定文件/版本引用；不自动抓外链；缩略图与真实二进制另验 | 原型已做，固定版本嵌入未闭 |
| Skill/模板/资料包 | DriveBridge resource descriptor + canonical Skill/Registry owner | 只读 descriptor、snapshot、check receipt；检查不代表安装、执行、授权或付费 | 原型已做，正式资源库已有只读链 |
| Pinax 本机阅读 | Pinax reader/desktop owner | Web 不读 `.pinax/**`；没有本机能力时显示 `unsupported`；写入走 proposal/confirm/apply | 原型仅有边界说明 |

原型中“页面是网盘里的 `.md` 文件”与正式实现中的“页面元数据由 Server 管理”并不冲突：正文、文件版本和字节仍由 DriveBridge/Pinax owner 管理；树关系、图标、日记日期和受保护属性通过公开元数据合同投影，不能由 renderer 自行推断并写回。

## 3. 正式数据与状态合同

所有请求都携带可验证的 `actor + tenant + space + source + object + revision/contextRevision`。页面目录和正文读取必须绑定当前 scope；换来源、撤权、离线或上下文代次变化时取消旧请求并清除敏感正文。

知识库至少区分以下状态：

| 状态 | 用户可见行为 | 禁止的降级 |
| --- | --- | --- |
| `loading` | 显示对象明确的骨架/进度 | 不以空列表代替加载中 |
| `empty` | 区分无来源、空目录、无 Markdown、无命中 | 不注入示例数据 |
| `partial` | 显示已扫描/不可用/过期范围 | 不冒全库计数或完整关系 |
| `stale` | 保留旧引用，要求重新查询或重开 | 不静默切到最新版本 |
| `forbidden` | 清除正文、片段、媒体和隐藏名称 | 不旁路文件读取或旧 token |
| `offline` | 保留有界本机草稿和已观察版本 | 不自动保存、同步或付费分析 |
| `conflict` | 并排显示 base/我的/服务端版本，要求选择 | 不强制覆盖或清草稿 |
| `unknown` | 回查原 operation/receipt 后再决定 | 不生成新幂等键重试 |
| `unsupported` | 说明缺失合同、宿主或格式 | 不用 mock 假装可用 |

保存、创建目录、创建页面、属性变更、资源登记和检查都必须复用原幂等键与原回执。目录树慢、响应丢失或刷新只允许 lookup 原请求；服务端拒绝时保留用户输入并给出 owner 修复位置。

知识库的目录层级直接读取 `/browser?space_id=<space>&path=<directory>`，只展示当前物理目录的直属子目录和 Markdown 文件。带幂等键的目录创建写入 `WorkspaceRequest`，通过 `/directories/lookup` 回查；无幂等键的旧 Finder 创建路径保持兼容。普通笔记创建增加 `directory_path`，服务端仍复用上传、版本和 metadata receipt 链。

创建弹层在发送请求前必须预检恢复能力。localStorage、Web Locks 或非本机普通 HTTP 安全上下文不可用时，页面保持拒写并显示具体原因；不得为了可用性改用无锁单标签写入。损坏的恢复槽不得自动删除，只有用户明确确认后，应用才能在锁内校验并清理该槽。服务端已确认成功但浏览器清理槽失败时，仍打开已创建对象并保留原 key，后续只允许查询原请求。

## 4. Skill、模板与资源边界

知识库可以展示 `SKILL.md`、模板和 reference pack 的目录、描述、文件成员、固定版本和检查状态，但 canonical Skill 仍归 `.skills/yeisme/` 或 Template Registry owner。知识库编辑器不得修改 Skill 的结构化 catalog、安装状态、权限、版本发布或 provider 配置。

用户在知识库中执行以下动作时，必须分开显示：

1. **读取**：打开固定版本正文或 descriptor；只读，不执行脚本。
2. **检查**：创建或回读 integrity/conformance check；结果只表示检查范围内的证据。
3. **提案**：把选定的固定引用交给 Skill/Registry owner；不代表安装或执行。
4. **安装/执行/发布**：由对应 owner 的 CLI/MCP 和授权门处理，知识库只显示回执或 `unsupported`。

资料包快照必须钉住成员 `file_id/version_id` 或 descriptor revision；更新 head 不得改变既有快照的阅读结果。资源正文、provider payload、凭据和临时 URL 不进入普通日志、MCP 文本或测试证据。

## 5. 页面数据库与媒体块

`pages` 代码块只声明一个视图，表格或看板选择保存在本机私有状态，不写进正文。卡片拖动或菜单改动必须转成 property owner 的明确变更，带 expected revision、审计和可撤销提示；只读协作、离线和未知结果禁用拖动。

图片使用 `![[path|caption]]` 或安全的 Markdown 图片语法。只读取当前 scope 内的文件/版本，按源宽高预留布局，不自动下载外链；缺失、无权、MIME 不支持、版本变化和二进制读取失败分别呈现。固定版本嵌入、缩略图和真实二进制上传在独立合同完成前保持 `planned` 或 `unsupported`。

## 6. 实施任务与验证入口

本续接复用既有 F25/F26、F65–F68、W201–W205，不新建第二套知识库任务系统：

- 页面目录、物理目录树和新建继续落在 `web/src/features/notebook` 与 Server notebook contract；旧页面 metadata 只作为兼容投影。
- Skill/模板/资料包继续落在正式 `features/resources`、resource descriptor/check owner 和 canonical Skill/Registry owner。
- Pinax reader/writer 继续由 desktop/Pinax change 负责；Web 不绕过受管范围。
- 原型只读回读先更新来源摘要，再更新本地 spec、任务和 Skill；OpenDesign 写入必须另行明确授权。

定向验证命令：

```bash
cd /home/yeshugen/workplace/yeisme-agent/cli/drivebridge
NODE_OPTIONS=--no-experimental-webstorage pnpm --dir web exec vitest run src/features/notebook/KnowledgePage.test.tsx src/features/notebook/CreateNotebookPageDialog.test.tsx src/features/notebook/NotebookDirectoryTree.test.tsx src/features/notebook/knowledge-files.test.ts
openspec validate drivebridge-personal-workspace-rebuild-v1 --strict --no-interactive
git diff --check
```

这些命令只验证本地正式实现和规格一致性；它们不证明真实 Pinax vault、真实云盘、Skill 安装/执行、Electron 三平台或发布部署。

## 7. Web 页面结构与恢复边界（2026-10-18）

DriveBridge Web 沿用 `WorkspaceApp`、`page-list`、`WorkspaceShell` 和现有 lazy route，不引入第二套路由或 `PageModule`。知识库与资源库是首批完整收口的 feature lane，页面职责按以下目录分层：

```text
web/src/features/notebook/
├── page.tsx           # 路由兼容入口
├── contract.ts       # actor/tenant/space/source/object/version/revision 与页面状态
├── projection.ts     # DTO 到安全 view model 的纯投影
├── state.ts          # 来源、目录、阅读和局部 Sheet 状态
├── clients/          # 现有 knowledge/page/document 合同适配
├── recovery/         # 草稿、CAS、原 operation/receipt 恢复
└── components/       # 页面树、目录、正文、属性与工作台组件

web/src/features/resources/
├── page.tsx           # 路由兼容入口
├── contract.ts       # descriptor、revision、snapshot、check 与登记回执
├── projection.ts     # 列表筛选、详情和检查历史投影
├── state.ts          # Inspector、检查、快照和登记阶段
├── clients/          # descriptor、check、snapshot HTTP 适配
├── recovery/         # 原幂等键、摘要槽、锁和回执恢复
└── components/       # 资源列表、登记、检查和成员快照组件
```

页面之间禁止导入彼此私有模块。固定资源成员阅读通过 `consumer-flows/public.ts` 的公开 reference entry 进入现有文件阅读流程；旧入口继续保留 re-export，迁移不改变 API、identity、`base_version_id`、CAS 或回执语义。页面状态只保存安全投影与局部 UI 状态，不保存 owner canonical content、凭据、Pinax vault 或 provider payload。
