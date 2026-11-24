# DriveBridge Client 视觉与交互

个人文件工作台。不是运营后台，也不是营销页。绿色工作区骨架见 `green-workspace` spec；壳层视窗与导航分组见 [客户端壳](client-shell.md)；本文件约束图标、令牌和 Finder 操作手感。

当前工作区去重、单层导航和共享空间配额规则见 [空间配额与工作区](capacity-workspace.md)。

当前工作区规范见 [紧凑工作区与可读性规范](compact-workspace.md)，对应 `drivebridge-compact-workspace-v1`。该规范已纳入两轮浏览器反馈并落实；尺寸与交互冲突以其明确修订为准，不将“紧凑”解释为减少文字内边距。

## 产品姿态

- 关键词：克制、精准、高信噪比、文件优先。风格名 **Quiet Precision（克制现代生产力风）**，对标 Linear / Raycast / Notion。
- 基底去偏色：浅色采用纯正无彩色极浅灰侧栏（`#f8f9fa`）配纯白无框一体文件画布（`#ffffff`），深色采用无彩色深炭黑基底（`#0c0d0e` / `#121416`）；去除全局绿偏色、高光渐变按钮与旋转漂浮插画，品牌松绿仅用于唯一主操作按钮、焦点环与关键状态点。
- 单侧栏一体壳层：去掉套娃外框与双搜索框打架；默认首屏直达首个本人个人网盘（`#/drives` 保留全量网盘目录）；侧栏顶部保留唯一实底主入口「＋ 新建 / 上传」+ 7 项高频入口（含网盘切换与常用收藏目录），低频运维模块统一收入「设置与空间管理」中心。
- 字体：`"SF Pro Text", "PingFang SC", system-ui`。文件/版本标识用等宽并默认收进详情折叠区；用量与表格数字用 `tabular-nums`。
- 英文 eyebrow 只出现在登录页；常规文件列表展示中文可读类型而非原始 MIME；前台文案统一采用简洁用户语言，避免裸露内部架构与免责术语。

## 令牌

颜色统一在 `src/app/theme/tokens.css` 与 `src/shadcn.css` 定义：shadcn 使用 `--ui-*`，现有布局变量 `--surface`、`--ink`、`--muted` 等指向同一套语义色。禁止在页面中写死白色表面或为深色模式复制一套组件。

| 语义 | 浅色 | 深色 |
| --- | --- | --- |
| 页面/主画布 | `#ffffff` | `#0c0d0e` |
| 卡片/浮层 | `#ffffff` | `#16181b` |
| 侧栏/次表面 | `#f8f9fa` | `#121416` |
| 主文字 | `#111315` | `#f2f4f7` |
| 次级文字 | `#5e646e` | `#98a0ab` |
| 主色 | `#186346` | `#4cc38a` |
| 边框 | `#e6e8ec` | `#23262b` |

样式层顺序固定为 `theme → base → legacy → components → utilities`，每个独立样式入口声明同一顺序，防止分包加载时 Tailwind reset 覆盖布局间距。新组件使用 shadcn 和 Tailwind；迁移中的布局样式仅在 `legacy` 层，颜色与新组件共用令牌。该层不表示全站组件迁移已经完成。

正文与次级文字在对应背景上保持对比度至少 4.5。焦点环可见。媒体图片、HTML 内容和标注画布不做反色处理；媒体信息面板跟随应用主题。

个人设置及偏好持久化规则见 [个人设置与主题](personal-settings.md)。

## 产品标

隐喻：桥（跨存储）+ 文件签（版本）。唯一手绘 SVG。

- 128px+：三墩 + 拱 + 顶签 + 底梁
- 32–64：加粗三竖，保留顶签
- 16–24：拱 + 底梁 + 顶签

侧栏、登录、favicon、apple-touch、PWA maskable 同源。应用内组件：`BrandMark`。

## 图标与克制文件徽标

通用工具图标基于 `lucide-react`（侧栏统一 `18px`、`strokeWidth={1.75}` 单色线性图标，无彩底高光方块）；文件列表与网格使用封装在 `web/src/components/file-art/` 下的克制扁平线性文档徽标 `FileSheetSvg`、轻量文件夹标识 `FolderGlyph` 与干净线性空态组件 `EmptyDropIllustration`（去除夸张投影与悬停歪头旋转，禁止在业务页面散落裸手绘 SVG）。

## 物理弹簧动效规范（UI Spec 准入例外）

正式准入 `motion`（`motion/react`）作为共享滑块（`layoutId`）、悬浮胶囊坞与卡片弹簧过渡引擎，并在 `AppearanceProvider` 顶层挂载 `<MotionConfig reducedMotion={preferences.motion === "reduced" ? "always" : "user"}>`，确保系统 `prefers-reduced-motion: reduce` 与用户「减少动效」设置一键静默全部位移动效。

- 侧栏 20px，`strokeWidth={1.75}`
- 工具条与菜单图标统一目标为 16px
- 图标按钮统一目标：桌面 32px，触控至少 44px；文件卡和预览缩略项不适用单行按钮高度
- 纯图标必须有 `aria-label`

导航不得复用同一图形：总览 `Gauge`、用量 `ChartColumn`、网盘 `HardDrive`、Agent `Bot`、团队 `Users`、用户 `ContactRound`、Key `KeyRound`。

文件类型走 `mediaKind()`：image / video / audio / pdf / text / converted / other。

文件夹封面 `FolderArt` 属于缩略图，不属于图标，只用于网格视图和空目录；目录树、列表行仍用 lucide `Folder`，仅按 `folderTone` 着色。

## 文件夹封面

网格视图的文件夹是内容预览容器，不是装饰插画：封面露出当前层最近修改的至多 3 个文件类型（文档、PDF、表格、图片、视频、音频、子文件夹），下方写文件与子文件夹数量，前板胶囊写本层文件的最近修改时间。实现见 `web/src/components/folder-art/`。

- 数据：进入视口才读该目录 `/browser` 第一页（`sort=date&order=desc`），同时最多 2 个请求，切换目录、视图或刷新时中止。超过一页写“N 项以上”，不宣称精确计数；读取失败写“内容暂不可读”。不为封面创建缩略图衍生物。
- 颜色：`--folder-tone-1..5`（松绿、湖蓝、蜂蜜、灰玫、银灰）按文件夹名哈希分配，同名同色，树、列表、网格一致；只表示身份，不表示状态。不用橙色、珊瑚色或紫色。
- 材质：工作台唯一的材质元素，即后板、纸张加单层磨砂前板；不叠第二层玻璃，周围界面保持扁平。`prefers-reduced-transparency` 去掉模糊。
- 动效：悬停、聚焦或有文件拖到文件夹（含空目录）上时前板前倾、纸张上浮，`--motion-standard`；`prefers-reduced-motion` 取消全部位移，只保留底色反馈。
- 可访问性：封面 `aria-hidden`；名称和数量是真实文本；时间胶囊由 CSS `attr()` 绘制，并以 `sr-only` 文本重复。

## Finder

页面模式：树 + 列表、网格、分栏或画廊 + 可选检查器。分栏/画廊的名称排版与完整阅读入口纳入本轮改进，不因已有视图按钮就宣称阅读体验已通过。

选择：单击替换，⌘ 单击切换，Shift 范围，双击预览或进入。细指针下 checkbox 不常驻。

键盘：↑↓ 焦点，→ 进文件夹，← 回上级，字母定位名称，Enter/空格预览，F2 重命名，Delete 回收站，⌘C/X/V/D/A 剪贴板与全选，Esc 清选择，⌘B 折叠主导航。

空目录本身就是落点，不再叠一条上传提示条。有文件时才显示紧凑拖入条。总览指标卡可点进网盘或传输。

菜单：细指针锚定；粗指针 action sheet。分组为打开、传输、整理、危险。

拖拽：文件夹与树节点在 `dragover` 时描边 `--green`。只读不接 drop。

动效：120–180ms，`transform`/`opacity`。`prefers-reduced-motion: reduce` 时取消位移。

## 壳层

已登录工作台视窗锁定、三组导航、资源态势 / 访问凭据二级 Tab、PWA overlay 见 [客户端壳](client-shell.md)。Finder 密度可低于壳层列表（行高以 Finder 为准），但颜色与焦点环必须同源。

## 团队邀请

协作组「团队」在有待处理邀请时显示数量。列表上方一张下一步卡，主按钮只做当前步骤。成员页一个「邀请加入」抽屉。开通后复制交付说明，不复制开放链接。令牌与壳层相同，不在 `teams.css` 堆 hex。

## 其它工作台页面

网盘和团队目录的列表详情模式、48px 最低表格行高、单元格 8px/12px 留白、表单操作区及分页归属，统一见 [本轮规范](compact-workspace.md)。现有个人密度设置不得把内容压到边框上。普通分隔线来自边框令牌；焦点环、选中态与行分隔线分开定义。

传输、收件箱、Agent、连接、分享共用同一套列表手感，不各自发明交互。

- 卡片可聚焦。↑↓ 或 j/k 移动，Enter 打开主操作，Delete 走危险确认，Esc 取消焦点。
- 空状态：一句说明 + 一个主按钮（去网盘、创建授权、查看传输）。
- 危险操作先确认。取消传输、撤销 Agent、撤销分享、断开连接都是可理解的后果，不是静默消失。
- 活动记录用中文动作名，不展示原始 `keys.disable` 点分串。
- 连接卡片按 `backendOf` 诚实写能力，不把 generic 说成可交接。

## 黑名单

紫渐变、玻璃拟态堆叠（文件夹封面的单层磨砂前板是唯一例外）、emoji 当图标、操作页大 hero、Inter 作主字体、为装饰引入动画库、把 Key 明文或 token 画进截图。

## 控件迁移验收

业务 Button、Input、Textarea、Checkbox、Select、Combobox 与 Dialog 统一使用 shadcn Base UI。业务表格使用 TanStack v9 DataTable，GridTable 是动态列适配器。已删除旧按钮、原生 select 视觉规则、directory-table、transfer-card 和原生 dialog 样式；剩余工作区/媒体布局 CSS 继续消费语义令牌。`pnpm check:ui` 解析 JSX 并扫描旧 CSS，阻止重复引入。
