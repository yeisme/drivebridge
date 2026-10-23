# DriveBridge 多笔记工作台正式实现规格

2026-10-04 · 设计到实现交接版

本文件把当前 OpenDesign 多笔记工作台原型收敛为 `cli/drivebridge/web` 的正式实现规格。它定义用户可观察行为、私有视图状态、前端模块边界、Pinax 适配边界和验收口径；不把 OpenDesign 编译产物当作生产源码，也不改变已有 HTTP/MCP 公开合同。

当前原型入口：

- [DriveBridge 原型](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/index.html)
- [动效与交互验收截图](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/review/notebook-motion-v1/index.html)
- [Pinax 接口映射](http://10.10.1.101:7456/api/projects/drivebridge-personal-agent-workspace/raw/pinax-notebook-contract-mapping.md)

合同依据：DriveBridge 文件与权限以 [Server 工作区合同](../../server/personal-workspace-contracts.md) 为准；Pinax 只读以 [人类阅读 API](../../../../pinax/docs/interfaces/human-reading-api.md) 为准，受监督写入以 [人类阅读写入 API](../../../../pinax/docs/interfaces/human-reading-write.md) 为准。

## 1. 产品边界与 owner

DriveBridge Web 负责工作台布局、文件导航、标签页、窗格、阅读位置、编辑焦点、私有视图偏好和渲染展示。DriveBridge Server/CLI 负责文件、版本、权限、固定引用、操作回执和可恢复写入。Pinax 负责 Markdown 正文、稳定身份、标签语义、关系和写入裁决。Electron 只提供受控宿主能力，不把领域状态搬进 renderer。

本轮正式实现包含：

- 网盘双侧栏与笔记工作台的统一壳层。
- 多标签页、固定/关闭/重新打开、两个并排窗格和可调分隔线。
- 普通 Markdown 与 Pinax 演示 adapter 的同一套阅读界面。
- 阅读、实时预览、源码三种模式；CodeMirror 负责源码事务与撤销。
- 共享大纲、双链、反向链接、块锚点、悬停预览、标题和列表折叠。
- 命令面板、快速打开、当前笔记搜索和当前来源全文搜索的范围区分。
- 日记、模板、图谱和附件入口的可交互空态、加载态、失败态。
- CSS 动效、减少动态效果、触控适配和键盘焦点恢复。

本轮不包含：

- 真实 Pinax vault 连接、生产写回或 `.pinax/**` 读取。
- 新的公开 HTTP/MCP endpoint、Electron 三平台实机接入和发布签名。
- 把 OpenDesign 的 `assets/personal-agents-notebook.js` 直接复制到生产构建。
- 用客户端 mock 绕过 Server/Pinax 的权限、revision 或 operation 语义。

## 2. 页面结构与响应式规则

桌面笔记页由产品导航、笔记目录树、正文工作区和关联面板组成。网盘的产品导航仍是第一侧栏；笔记目录树只管理当前来源的文件、标签和日记，不重复承担产品导航职责。

知识库的目录树以当前 DriveBridge Space 的物理网盘目录为唯一层级来源。目录展开读取 `/browser`，新建目录调用 `POST /directories`，新建笔记调用 `POST /notebook/pages` 并传入 `directory_path`；成功后必须刷新当前目录并打开新笔记。`parent_file_id` 仍可被旧页面 metadata 合同读取，但不再驱动新建入口。

| 容器宽度 | 默认结构 | 收缩顺序 |
| --- | --- | --- |
| ≥1280px | 产品导航 + 笔记树 208px + 正文 + 关联栏 224px | 先隐藏次要按钮文字 |
| 1024–1279px | 产品导航 + 笔记树 + 正文，关联栏默认覆盖 | 关联栏转抽屉，再收缩笔记树 |
| 768–1023px | 产品导航可收起，笔记树和关联栏均可抽屉化 | 正文保持单栏 |
| <768px | 单列正文，底部导航，两个侧栏分别抽屉 | 详情面板全屏打开 |

正文宽度取当前窗格可用空间和用户选择行宽的较小值。默认 16px、行高 1.75；字号支持 14–24px，行宽支持窄/标准/宽三档，最大值分别为 640/720/880px。代码、表格、公式和 Mermaid 只在自己的容器内横向滚动。

工具栏固定为：

```text
32px 标签页栏
40px 文档工具栏
编辑模式额外显示 32px 格式栏
正文与底部文档状态栏
```

阅读模式隐藏格式栏；标题、版本状态和模式切换必须保持单行。空间不足时依次隐藏文字、收进更多菜单，不能让标题换行推动正文。

## 3. 私有视图状态

私有状态以用户、空间、来源、对象和 revision 隔离。它不进入 Markdown 正文、Pinax 标签或公开 API。

```ts
type NotebookViewState = {
  tabs: TabState[];
  activeTab: string;
  secondTab?: string;
  split: boolean;
  splitRatio: number;
  leftOpen: boolean;
  rightOpen: boolean;
  leftWidth: number;
  rightWidth: number;
  readingWidth: "narrow" | "standard" | "wide";
  fontSize: number;
  focus: boolean;
  outlineTab: "outline" | "relations" | "properties";
  expandedTags: string[];
};

type TabState = {
  key: string;
  sourceId: string;
  objectId: string;
  revision: string;
  pinned: boolean;
  dirty: boolean;
  scrollTop: number;
  selection?: { from: number; to: number };
  collapsedAnchors: string[];
};
```

恢复规则：

- 不同对象、来源或 revision 不能共享草稿、滚动位置和折叠状态。
- 同一对象同一 revision 在两个窗格共享草稿，但各自保留选区和滚动位置。
- 历史版本永远只读；返回当前草稿时只能恢复相同身份和 base revision 的草稿。
- 不同 actor、space 或 source 的本地存储键必须隔离；敏感正文草稿默认留在内存。
- 创建目录/笔记的恢复槽还必须隔离 tenant、物理目录和动作类型；初始化时区分存储不可用、Web Locks 不可用、非安全上下文与损坏记录。未知结果只允许原 key lookup/resume，不能生成替代 key。
- 清除私有偏好只重置视图，不删除文件、版本、标签或 Pinax 数据。

## 4. 导航与编辑交互

### 标签页与窗格

- 普通单击笔记复用未固定预览页；双击或进入编辑后固定。
- 关闭有草稿的标签页先确认；取消关闭保持原焦点和滚动位置。
- 关闭后可从“已关闭笔记”重新打开；固定标签不因普通切换消失。
- `Cmd/Ctrl+Tab` 循环切换已打开标签页；切换后焦点落在目标标签。
- 两个窗格最小宽度 480px；空间不足时仅显示当前窗格，另一窗格通过切换入口访问。
- 分隔线拖动期间直接跟随指针，释放后最多 120ms 吸附；不得在拖动中重建文档。

### 命令与搜索

- `Cmd/Ctrl+P` 打开命令面板。
- `Cmd/Ctrl+O` 打开快速打开。
- `Cmd/Ctrl+K` 保留全局文件搜索。
- `Cmd/Ctrl+F` 在编辑器内只搜索当前编辑范围；当前来源全文搜索必须显示范围标签。
- 输入框、CodeMirror、富文本编辑器和中文输入法组合期间，以上全局快捷键让位。
- Escape 按层级关闭菜单、悬停预览、详情面板、专注模式，最后返回触发控件。
- 命令面板不主动扩大授权范围；结果只来自当前用户可访问和已加载的来源。

### Markdown 编辑

编辑器必须保留 CodeMirror 事务、撤销/重做、选择、输入法组合和保存状态。实时预览和阅读模式共享同一解析结果、引用解析和 anchor 映射。

支持的 Markdown 能力：

- 标题、列表、任务、表格、代码高亮、脚注和 Callout。
- 公式使用按需加载 KaTeX，`trust: false`。
- Mermaid 使用 `securityLevel: "strict"`，不允许正文配置覆盖安全级别。
- 双链支持别名、标题引用、块引用、反向链接和未解析目标提示。
- 嵌入最多展开两层；循环、失权、缺失和版本不可达均显示占位和返回来源入口。
- 原始 HTML 不执行，外部图片不自动联网；渲染失败保留源码和重试入口。
- 重复标题生成稳定且不冲突的 anchor；大纲、搜索、引用跳转和滚动恢复共用 anchor 映射。

### 块级交互

- 标题和列表区块支持折叠，折叠状态按对象/revision 私有保存。
- 桌面鼠标悬停或键盘聚焦时显示区块工具条：复制链接、复制引用、折叠。
- 触控设备不自动显示悬停工具条；折叠按钮触点至少 44px。
- 源码模式不注入渲染工具条；正文 DOM 不承载领域状态。
- 本轮不提供块拖拽排序，避免扩大写入冲突和撤销范围。

## 5. 动效合同

不新增动画运行库，使用 CSS transitions、keyframes 和已有状态属性。

| 场景 | 动效 | 约束 |
| --- | --- | --- |
| Hover/active/focus | 120ms 颜色、边框、透明度 | 不改变密集列表行高 |
| 菜单、命令面板、抽屉 | 180ms 淡入 + 4–8px 位移 | Escape 和 outside click 可立即关闭 |
| 预览卡片、模式切换 | 220ms 轻微缩放 + 淡入 | 不阻塞输入，不制造布局跳动 |
| 区块折叠 | 180ms 高度/透明度过渡 | 内容终态明确，焦点不落入隐藏区块 |
| 分隔线拖动 | 拖动时即时，释放后短暂吸附 | 不在拖动中重建正文 |
| 保存状态 | 颜色/图标过渡 | 不使用无限 spinner 或闪烁 |

`prefers-reduced-motion: reduce` 和用户动效偏好必须取消非必要位移、缩放和连续动画，但保留选中、焦点、错误、保存和内容层级反馈。

## 6. 异步、权限与错误状态

所有请求带 `actor + space + source + object + revision + contextRevision`。新上下文开始时取消旧请求；晚响应不能覆盖新笔记、新版本或新窗格。

| 状态 | 行为 |
| --- | --- |
| loading | 结构稳定的骨架，不清空已确认正文 |
| empty | 区分无来源、空目录、无搜索结果和无反向链接 |
| partial | 正文、关系、附件和索引独立展示失败原因 |
| stale | 显示旧 revision 和“重新读取”，不静默换到最新版 |
| forbidden | 清除正文和敏感元数据，不用无限加载代替拒绝 |
| offline | 保留有界只读缓存，写入进入等待或明确不可用 |
| conflict | 保留草稿和服务端版本，要求差异确认 |
| unknown | 先通过原 operation/receipt 回查，禁止换幂等键重试写入 |
| unsupported | 显示缺失能力及安全替代，不使用 mock 假装成功 |

保存必须遵守：草稿 → 差异预览 → 用户确认 → owner 应用 → 回执。64KiB writer 正文预算超限时变为只读并说明原因；原文导出不代替编辑原文接口。

## 7. 正式代码模块与适配器

正式实现进入 `web/src/`，原型仅作为行为参考。建议模块边界：

```text
src/features/notebook/
  NotebookWorkspace.tsx       # 页面壳、响应式布局、面板和窗格
  notebook-state.ts            # 标签页、窗格、焦点、私有偏好
  notebook-session.ts          # 草稿、revision、滚动与选区
  notebook-renderer.tsx        # Markdown/GFM/KaTeX/Mermaid/安全渲染
  notebook-navigation.tsx      # 命令面板、快速打开、大纲、反向链接
  notebook-blocks.tsx          # 折叠、区块工具条、块引用
  notebook-adapters.ts         # 普通 Markdown 与 Pinax 合同适配器
  notebook-workspace.css       # 动效、响应式和阅读令牌
```

适配器只转换 owner 合同，不持有权限判断或领域持久化：

- `MarkdownFileAdapter`：消费 DriveBridge 文件/版本/预览合同。
- `PinaxReaderAdapter`：消费 Pinax 已有 reader 合同；真实合同未提供的能力显示 `unsupported`。
- `PinaxWriterAdapter`：只生成 proposal、diff、confirm、apply、receipt 流程，不旁路普通文件覆盖。

不得从生产模块导入 `web/prototypes/personal-agent-workspace/model.ts` 的演示状态、样本数据或 reset 逻辑。

## 8. 分阶段实施

1. **Web 壳与私有状态**：实现双侧栏、标签页、双窗格、响应式抽屉、阅读位置和焦点恢复。
2. **统一 Markdown 渲染**：建立共享解析结果、anchor 映射、CodeMirror/阅读/实时预览三模式。
3. **Obsidian 式交互**：命令面板、快速打开、悬停预览、反向链接、标题折叠、区块工具条。
4. **真实文件合同**：接入文件列表、固定版本、权限、分享和回执；移除演示数据依赖。
5. **Pinax 适配**：先 reader，再 writer proposal/confirm/apply；未实现能力保持 unsupported。
6. **验收与交接**：桌面/手机/深色/reduced-motion、键盘、中文输入、失败恢复和原网盘回归。

## 9. 验收命令与完成口径

从 `cli/drivebridge/web` 执行：

```bash
pnpm build
pnpm prototype:unit
pnpm prototype:test
```

正式实现还必须补充：

- 阅读/实时/源码模式、撤销重做和中文输入测试。
- 多标签、分屏、焦点恢复、滚动恢复和私有状态重启测试。
- 双链、反链、块引用、重复标题、循环嵌入和渲染失败测试。
- reader/writer 的权限撤销、revision 冲突、未知结果和 64KiB 超限测试。
- 320/390/768/1024/1440/1920px、浅深色、200% 缩放和 reduced-motion 浏览器测试。
- 原网盘四视图、多选、快速预览、分享、Agent 授权和任务确认回归。

OpenDesign 截图和 HTTP 原型验证可以作为设计证据，但不能替代真实 Server/Pinax、Electron 或生产部署证据。

## 10. Notion 参考阅读预览（2026-10-09）

本轮按用户要求参考 [Notion 页面样式与宽度](https://www.notion.com/help/customize-and-style-your-content)，保留 DriveBridge 的 Precision Pine 色彩与公共阅读器，不复制独立笔记产品或更换权限模型。

- 正文采用无嵌套框线的文档布局、明确标题层级、段落/引用/表格间距；章节与列表折叠仍可键盘操作。
- 工具栏新增“阅读设置”：14–24px 字号、640/720/880px 三档行宽，复用 actor/space/source 隔离的私有偏好；不修改正文。320px 七个工具触点保持44px。
- 普通文件信息栏接入真实收藏、文内链接与反向链接；固定来源/版本独立呈现。关系只覆盖显式建立索引且当前可访问的文件，不把未索引或未知覆盖度解释为空库。
- 索引重建仅用户触发，复用固定版本字节，不抓取外链或调用模型；原键恢复与服务端权限过滤保持。收藏删除仅删除绑定，重新收藏使用 tombstone 的连续修订。
- 公式按需加载 KaTeX，`trust:false`；图表按需加载 Mermaid，`securityLevel:strict`、禁HTML标签。拒绝正文配置、HTML、资源URL与自定义样式，安全失败保留源码。每篇最多200个公式、公式单项4096字符/合计32768字符、8个图表，每图32000字符/500条边；超预算保留源码。
- 普通文档冲突可先查询原保存结果，再读取当前固定版本并人工合并；新基线和差异须明确确认。未知操作不能换键，正文只在内存，历史发布版本保留。

实际本地证据：`temp/integration-test-runs/20261008T174345Z-83114a09/` 覆盖六视口浅深色、阅读设置、真实公式/图表、收藏取消重加、固定关系索引及反链；`20261008T174051Z-3f9f6c9f/` 覆盖两个浏览器标签的连续冲突、人工合并与全部旧版本回读。截图位于前者 `artifacts/`。不是 Pinax、原生三平台或全站最终验收。
