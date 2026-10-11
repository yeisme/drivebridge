## Why

绿色工作区、产品标和 Finder 手感已经落地，但壳层仍像网页后台：12 个侧栏入口平铺、Insights 柱图是裸 SVG 色块、数字与边框各写各的 hex。用户不知道如何合理拓展，也不知道怎样把同一份资产当成客户端而不是 CMS。现在收口壳层风格、图表原语和导航分组，不改存储合同，也不引入原生壳。

## What Changes

- 把 `Precision Pine` 令牌升为壳层真源：表面阶梯、微边框、投影、等宽数字、主按钮内高光。业务 CSS 不再堆新 hex。
- 已登录工作台视窗锁定：外层不滚，侧栏 / 主区 / 检查器各自滚动；顶栏固定为一行工具栏。
- 主导航改成三组（工作台、协作、系统）可折叠；折叠态保持图标 Rail。总览与用量共用「资源态势」入口，用户目录与 Key 共用「访问凭据」入口。
- Insights 图表抽出轻量 SVG 原语：圆角柱、细网格、跟随浮层、缺失日仍是缺口不是零。不引入 ECharts/Chart.js。
- 服务端网页 PWA 增加 `window-controls-overlay` 与桌面快捷方式；本机 `drivebridge view` 仍不声明安装清单。文档写清三阶段客户端路径：PWA → 轻量宿主评估 → 托盘/文件协议，当前只做第一阶段。

## Capabilities

### New Capabilities

- `client-shell`: 视窗锁定、分组导航、壳层令牌、PWA 标题栏融合与客户端扩展边界。
- `insight-charts`: 诚实、可键盘操作的轻量图表原语与容量分段条。

### Modified Capabilities

- `green-workspace`: 折叠侧栏继续成立，但入口按三组呈现，总览/用量与用户/Key 合并为二级 Tab。
- `visual-craft`: 令牌扩展为 Precision Pine 表面阶梯；按钮、状态胶囊、卡片使用同一材质。
- `insights-audit`: 图表交互从底栏读数升级为跟随浮层，数据表与缺失日语义不变。
- `drivebridge-personal-web`: 可安装清单声明 overlay 与快捷方式，仍禁止独立客户端壳。

## Impact

fit：仅 `client/drivebridge-client` 视觉、导航、图表组件、PWA 清单与 Playwright 截图。split-owner：用量/审计数据仍在 backend-server/drivebridge 与 mcp/drivebridge。不改 API、权限、Key 明文规则。无新 npm 依赖。回滚只还原前端资源，服务合同保持可用。
