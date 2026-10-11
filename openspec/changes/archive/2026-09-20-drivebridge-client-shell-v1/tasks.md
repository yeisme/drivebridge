# Tasks: drivebridge-client-shell-v1

范围：`client/drivebridge-client`。不变更 backend-server 或 MCP 合同。界面中文，字段名英文。Lane A（令牌/壳层）与 Lane B（图表原语）可并行；导航合并依赖 A。

## 1. Precision Pine 令牌与文档

- [x] 1.1 在 `src/style.css` `:root` 补齐表面阶梯、微边框、投影、`--radius-xs`；主按钮用令牌与 1px 内高光；数字默认 `tabular-nums`。`insights.css` / `finder.css` / `workspace-layout.css` 能引用令牌的 hex 一律改引用。
  - owner: client；范围：`src/style.css`、`src/finder.css`、`src/insights/insights.css`、`src/workspace-layout.css`、`src/forms.css`。依赖：无。Lane A。
- [x] 1.2 更新 `docs/ui/DESIGN.md`，新增 `docs/ui/client-shell.md`（三组导航、视窗锁定、PWA overlay、非目标：Tauri/⌘K/暗色）。`docs/README.md`、`docs/cross-platform-client.md`、`docs/insights-audit.md` 链到本 change。
  - owner: client；范围：`docs/ui/`、`docs/README.md`、`docs/cross-platform-client.md`、`docs/insights-audit.md`。依赖：1.1 可并行写文档，落地值以 1.1 为准。

## 2. 视窗锁定与顶栏

- [x] 2.1 已登录 `.app` 使用 `height: 100dvh; overflow: hidden`；侧栏、`.content`、检查器各自滚动。登录页与本机 view 保持文档流。
  - owner: client；范围：`src/style.css`、`src/workspace-layout.css`、`src/finder.css`、`src/main.tsx`。依赖：1.1。Lane A。
- [x] 2.2 顶栏保持单行 56px：面包屑、传输胶囊、治理条不把主区顶出视口；窄屏不把分组标题塞进底栏。
  - owner: client；范围：`src/workspace-layout.css`、`src/style.css`。依赖：2.1。

## 3. 分组导航与入口合并

- [x] 3.1 主导航改为工作台 / 协作 / 系统三组，组折叠写入 `drivebridge:nav-groups`；解析失败当全展开。折叠 Rail 仍用互不相同 lucide 与 `aria-label`。
  - owner: client；范围：`src/main.tsx`、`src/style.css`、`src/finder.css`。依赖：2.1。
- [x] 3.2 资源态势合并 `insights` 与 `analytics` 为二级 Tab；访问凭据合并 `members` 与 `users`。`workspaceRoute` 将 `#/analytics`、`#/users` 映到对应 Tab。侧栏不再同时列出四项。
  - owner: client；范围：`src/main.tsx`、`src/workspace-route.ts`、`src/Workspace.tsx`。依赖：3.1。
- [x] 3.3 路由与分组单测：旧 hash、组折叠偏好、权限过滤后组不留空标题。
  - owner: client；范围：`src/workspace-route.test.ts`、`src/Workspace.test.tsx` 或新增 `src/nav-groups.test.ts`。依赖：3.2。
  - 验证：`pnpm exec vitest run src/workspace-route.test.ts`

## 4. 轻量图表原语

- [x] 4.1 抽出 `src/insights/charts.tsx`：`Trend` 圆角柱、hairline、缺失日缺口、并排有效/观测柱、键盘聚焦浮层。不新增依赖。
  - owner: client；范围：`src/insights/charts.tsx`、`src/insights/Insights.tsx`、`src/insights/insights.css`。依赖：1.1。Lane B，可与第 2、3 组并行。
- [x] 4.2 `MeterBar` 分段容量条：当前 / 历史 / 回收站 / 预留；`quota_bytes` 为空时不画满额。Insights 与侧栏用量能复用则复用，否则 Insights 先落地。
  - owner: client；范围：`src/insights/charts.tsx`、`src/insights/Insights.tsx`。依赖：4.1。
- [x] 4.3 图表单测：缺失日、并排柱、审计次数格式、聚焦读数、未知配额。
  - owner: client；范围：`src/insights/Insights.test.tsx` 或 `src/insights/charts.test.tsx`。依赖：4.1、4.2。
  - 验证：`pnpm exec vitest run src/insights/Insights.test.tsx`

## 5. PWA overlay

- [x] 5.1 `public/manifest.webmanifest` 增加 `display_override: ["window-controls-overlay","standalone"]`，以及网盘 / 传输 shortcuts。`theme_color` / `background_color` 对齐令牌。insights.html 不加 manifest。
  - owner: client；范围：`public/manifest.webmanifest`、`index.html`。依赖：1.1。可与第 3、4 组并行。
- [x] 5.2 不支持 overlay 时不出现死空白交通灯占位。文档写明回退 `standalone`。
  - owner: client；范围：`src/style.css`、`docs/cross-platform-client.md`。依赖：5.1、2.2。

## 6. 验证

- [x] 6.1 单测：`pnpm exec vitest run src/workspace-route.test.ts src/insights/Insights.test.tsx`
  - 预期：全部通过。失败复查：先看缺失日/旧 hash 断言是否被浮层 DOM 改坏。
- [x] 6.2 `pnpm exec vite build`（或在无并行脏文件时 `pnpm build`）。
  - 预期：产物含更新后的 manifest。失败复查：区分本切片与工作区其它未跟踪 ts 错误。
- [x] 6.3 Playwright：`tests/workspace-design.spec.ts`、`tests/insights.spec.ts`、`tests/workspace.spec.ts`。截图改为视口 + 主区，避免 fullPage 在锁定视窗下误导。
  - 验证：`pnpm exec playwright test tests/workspace-design.spec.ts tests/insights.spec.ts tests/workspace.spec.ts`
  - 证据：`temp/integration-test-runs/<run-id>/`。覆盖折叠组、`#/analytics`、图表键盘、390 宽无横向溢出。
