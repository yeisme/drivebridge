# DriveBridge Client 视觉与交互

个人文件工作台。不是运营后台，也不是营销页。绿色工作区骨架见 `green-workspace` spec；壳层视窗与导航分组见 [客户端壳](client-shell.md)；本文件约束图标、令牌和 Finder 操作手感。

当前工作区去重、单层导航和共享空间配额规则见 [空间配额与工作区](capacity-workspace.md)。

当前工作区规范见 [紧凑工作区与可读性规范](compact-workspace.md)，对应 `drivebridge-compact-workspace-v1`。该规范已纳入两轮浏览器反馈并落实；尺寸与交互冲突以其明确修订为准，不将“紧凑”解释为减少文字内边距。

## 产品姿态

- 关键词：克制、可扫读、可信、文件优先。风格名 Precision Pine。
- 保留绿色品牌。浅色主色 `#245a45`，深色主色 `#88d5ae`；状态色与图表色随主题切换。
- 字体：`"SF Pro Text", "PingFang SC", system-ui`。文件/版本标识用等宽；用量与表格数字用 `tabular-nums`。
- 英文 eyebrow 只出现在登录页。

## 令牌

颜色统一在 `src/shadcn.css` 定义：shadcn 使用 `--ui-*`，现有布局变量 `--surface`、`--ink`、`--muted` 等指向同一套语义色。禁止在页面中写死白色表面或为深色模式复制一套组件。

| 语义 | 浅色 | 深色 |
| --- | --- | --- |
| 页面背景 | `#f6f8f6` | `#111916` |
| 卡片/表面 | `#ffffff` | `#1a2520` |
| 浮层 | `#ffffff` | `#202e27` |
| 侧栏 | `#e4ece7` | `#15221b` |
| 主文字 | `#24382d` | `#e4efe8` |
| 次级文字 | `#4b6254` | `#a5b9ad` |
| 主色 | `#245a45` | `#88d5ae` |
| 边框 | `#d9e3dc` | `#34463b` |

样式层顺序固定为 `theme → base → legacy → components → utilities`，每个独立样式入口声明同一顺序，防止分包加载时 Tailwind reset 覆盖布局间距。新组件使用 shadcn 和 Tailwind；迁移中的布局样式仅在 `legacy` 层，颜色与新组件共用令牌。该层不表示全站组件迁移已经完成。

正文与次级文字在对应背景上保持对比度至少 4.5。焦点环可见。媒体图片、HTML 内容和标注画布不做反色处理；媒体信息面板跟随应用主题。

个人设置及偏好持久化规则见 [个人设置与主题](personal-settings.md)。

## 产品标

隐喻：桥（跨存储）+ 文件签（版本）。唯一手绘 SVG。

- 128px+：三墩 + 拱 + 顶签 + 底梁
- 32–64：加粗三竖，保留顶签
- 16–24：拱 + 底梁 + 顶签

侧栏、登录、favicon、apple-touch、PWA maskable 同源。应用内组件：`BrandMark`。

## 图标

lucide-react。禁止手绘业务图标。

- 侧栏 20px，`strokeWidth={1.75}`
- 工具条与菜单图标统一目标为 16px
- 图标按钮统一目标：桌面 32px，触控至少 44px；文件卡和预览缩略项不适用单行按钮高度
- 纯图标必须有 `aria-label`

导航不得复用同一图形：总览 `Gauge`、用量 `ChartColumn`、网盘 `HardDrive`、Agent `Bot`、团队 `Users`、用户 `ContactRound`、Key `KeyRound`。

文件类型走 `mediaKind()`：image / video / audio / pdf / text / converted / other。

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

紫渐变、玻璃拟态堆叠、emoji 当图标、操作页大 hero、Inter 作主字体、为装饰引入动画库、把 Key 明文或 token 画进截图。

## 控件迁移验收

业务 Button、Input、Textarea、Checkbox、Select、Combobox 与 Dialog 统一使用 shadcn Base UI。业务表格使用 TanStack v9 DataTable，GridTable 是动态列适配器。已删除旧按钮、原生 select 视觉规则、directory-table、transfer-card 和原生 dialog 样式；剩余工作区/媒体布局 CSS 继续消费语义令牌。`pnpm check:ui` 解析 JSX 并扫描旧 CSS，阻止重复引入。
