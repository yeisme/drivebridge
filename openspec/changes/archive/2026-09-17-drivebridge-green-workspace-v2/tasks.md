# Tasks: drivebridge-green-workspace-v2

范围：`client/drivebridge-client`（React/TypeScript/Vite）。服务端合同归 `backend-server/drivebridge`，本机接口归 `mcp/drivebridge`，本 change 不变更。所有界面文案中文，命令、flag、路径、字段名保持英文。

完成状态说明：以下任务已由 evidence runner 执行并写入 `temp/integration-test-runs/20260916T164311Z-91bbb935/summary.json`，统一标记为完成；任一项回归失败时取消勾选并按对应分组的失败复查步骤处理。

## 1. 绿色统一主题与两层网盘导航（Requirement: Two-level drive navigation）

Lane A（主题样式）与 Lane B（导航结构）可并行：A 只改样式层，B 只改路由/组件结构，汇合后由 1.4 联合验收。

- [x] 1.1 统一绿色主题到边框、卡片、工具栏、表格与弹窗，参考 shadcn Sidebar 分区与 icon collapse 交互，复用现有 React 组件。
  - owner: client；范围：`src/workspace-layout.css`、`src/style.css`、`src/finder-browser.css`、`src/finder.css`、`src/Workspace.tsx`、`src/Finder.tsx`、`src/FinderMenu.tsx`。依赖：无。
- [x] 1.2 实现“我的网盘”两层导航：第一层按个人／团队／存储展示可访问网盘卡片，第二层才挂载目录树与文件工作区；卡片明确团队与存储归属；进入空间后提供“全部网盘”返回、位置、目录树、文件列表与预览。
  - owner: client；范围：`src/Workspace.tsx`、`src/workspace-route.ts`、`src/Finder.tsx`、`src/Media.tsx`。依赖：1.1。
- [x] 1.3 本机 `drivebridge view` 投影（`src/LocalWorkspace.tsx`、`src/insights/`）同步两层入口与绿色主题；切换团队通过既有 tenant/space context 重新挂载隔离状态，不用客户端筛选模拟权限。
  - owner: client；范围：`src/LocalWorkspace.tsx`、`src/workspace-route.ts`。依赖：1.2。
- [x] 1.4 两层导航单元与组件测试：卡片渲染、进入/返回、团队与存储标签、未识别存储类型诚实降级。
  - owner: client；范围：`src/Workspace.test.tsx`、`src/workspace-route.test.ts`。依赖：1.2、1.3。
  - 验证：`pnpm test -- src/Workspace.test.tsx src/workspace-route.test.ts`
  - 预期：全部用例通过，无新增警告。失败复查：先跑 `pnpm test` 排除并发用例互相影响，再对照 `src/workspace-route.ts` 路由表确认测试路径与实现一致。

## 2. 导航折叠、移动端与可访问性（Requirement: Independent sidebar collapse）

Lane C 可与第 1 组并行（折叠逻辑独立于导航内容）。

- [x] 2.1 主导航与目录树独立折叠：折叠保留图标与无障碍名称（`aria-label`），`Ctrl/Cmd+B` 切换主导航，目录树有独立折叠按钮，展开后文件工作区正确收回。
  - owner: client；范围：`src/Workspace.tsx`、`src/workspace-layout.css`。依赖：1.2。
- [x] 2.2 两项折叠状态只保存非敏感 UI 偏好（不写入凭据/token）；移动端为图标导航加可展开目录树，窄屏不挤压文件区。
  - owner: client；范围：`src/workspace-layout.css`、`src/Workspace.tsx`。依赖：2.1。
- [x] 2.3 键盘与拖拽回归：折叠状态下目录树拖放、键盘导航、`Enter`/`Space` 操作与焦点可见性不回归。
  - owner: client；范围：`src/Finder.tsx`、`src/FinderMenu.tsx`。依赖：2.1。
- [x] 2.4 桌面与移动端布局 e2e：图标侧栏、两层导航、独立树折叠、拖放与键盘操作。
  - owner: client；范围：`tests/workspace-design.spec.ts`、`tests/workspace.spec.ts`、`tests/finder-design.spec.ts`。依赖：2.1–2.3。
  - 验证：`pnpm test -- src/Workspace.test.tsx`；`pnpm test:e2e -- tests/workspace-design.spec.ts`
  - 预期：组件测试与 e2e 全绿，截图含折叠/展开两态。失败复查：查看 `temp/integration-test-runs/<run-id>/artifacts/` 截图与 `stderr.log`，区分实现回归与 Playwright 环境（先 `pnpm build` 确认可构建）。

## 3. Dashboard 真实数据与空状态（Requirement: Honest dashboard data）

Lane D 可与第 1、2 组并行（Dashboard 数据层独立）。

- [x] 3.1 Dashboard 展示实际容量、传输、Agent 状态和当前网盘最近文件；原分析面板保留日期筛选；所有数字来自服务端真实计量。
  - owner: client；范围：`src/Workspace.tsx`、`src/insights/`、`src/api.ts`。依赖：1.2（当前网盘上下文）。
- [x] 3.2 诚实状态矩阵：团队与账号未启用、接口不可用、空网盘、传输为空时展示真实空态/错误态，不虚构成员或数据。
  - owner: client；范围：`src/Workspace.tsx`、`src/api.ts`、`src/i18n.ts`。依赖：3.1。
- [x] 3.3 进度与失败恢复：传输列表展示进行中进度，失败任务提供重试/取消入口；网络中断后恢复轮询。
  - owner: client；范围：`src/api.ts`、`src/History.tsx`（传输历史复用）。依赖：3.1。
- [x] 3.4 Dashboard 组件测试与 e2e：真实数据渲染、空态、服务不可用、本地 view 投影。
  - owner: client；范围：`src/Workspace.test.tsx`、`tests/workspace-design.spec.ts`、`tests/insights.spec.ts`、`tests/local-insights.spec.ts`。依赖：3.1–3.3。
  - 验证：`pnpm test -- src/Workspace.test.tsx`；`pnpm test:e2e -- tests/insights.spec.ts tests/local-insights.spec.ts`
  - 预期：空态/错误态用例断言无虚构数据文案。失败复查：mock 响应与 `src/api.ts` 字段对齐后重跑；仍失败则检查 `insights.html` 独立入口是否被路由改动影响。

## 4. Key 目录与身份关系展示（Requirement: Key identity visibility）

Lane E 依赖服务端 Key 元数据 API（`backend-server/drivebridge` 公共合同已就位），界面实现与测试可在本 lane 内并行推进后统一验收。

- [x] 4.1 Key 独立入口：搜索、状态筛选、稳定标识（可复制公开 ID，明确“不能拿来登录”）、用户映射、当前登录提示、命名、详情，保留既有撤销归档流程。
  - owner: client；范围：`src/KeyDirectory.tsx`、`src/api.ts`、`src/i18n.ts`。依赖：无（消费既有 API）。
- [x] 4.2 身份关系顺序展示 Key → 用户 → team 成员关系 → 网盘权限；标签不改变用户名称或权限；治理模式仅展示本人 Key 元数据，其他用户 Key 数量标注为当前可见范围；无匹配 Key、已归档均有明确状态。
  - owner: client；范围：`src/KeyDirectory.tsx`、`src/UserDirectory.tsx`、`src/Teams.tsx`。依赖：4.1。
- [x] 4.3 Agent token 一次显示与本人登录 Key 再次取回回归：明文依旧只在创建/领取界面出现一次；复制交互不回归；不把任何 token 写入 localStorage。
  - owner: client；范围：`src/KeyDirectory.tsx`、`tests/key-copy.spec.ts`、`tests/keylogin.spec.ts`。依赖：4.1。
- [x] 4.4 Key 目录组件测试与 e2e：改名后所属用户不变且不显示凭据明文、搜索/筛选、复制标识、撤销归档。
  - owner: client；范围：`src/api.test.ts`、`tests/key-copy.spec.ts`、`tests/keylogin.spec.ts`、`tests/users-management.spec.ts`。依赖：4.1–4.3。
  - 验证：`pnpm test -- src/api.test.ts`；`pnpm test:e2e -- tests/key-copy.spec.ts tests/keylogin.spec.ts`
  - 预期：无明文泄露断言通过；页面快照不含 token 值。失败复查：确认测试未把真实凭据写入 fixtures；检查 `temp/integration-test-runs/<run-id>/stdout.log` 是否有意外明文输出并按根仓库凭据政策处理。

## 5. 用户／团队与治理表单（Requirement: Honest dashboard data，团队场景）

Lane F 可与 Lane E 并行（团队视图与 Key 视图仅共享 `src/api.ts` 类型）。

- [x] 5.1 用户与团队独立入口：团队治理表单以可见用户选择器代替手填 ID，数据权限仍在服务端校验；未启用团队模式时显示真实空状态，不声称已配置团队。
  - owner: client；范围：`src/Teams.tsx`、`src/UserDirectory.tsx`、`src/teams.css`。依赖：1.2。
- [x] 5.2 权限/连接失效处理：会话失效、403、Provider 连接断开时给出可操作的重新登录/重连提示，不清空用户未提交表单。
  - owner: client；范围：`src/api.ts`、`src/Teams.tsx`。依赖：5.1。
- [x] 5.3 团队布局与成员视图 e2e：用户选择器、空态、失效提示。
  - owner: client；范围：`tests/teams.spec.ts`、`tests/teams-layout.spec.ts`、`tests/users-management.spec.ts`。依赖：5.1、5.2。
  - 验证：`pnpm test:e2e -- tests/teams.spec.ts tests/teams-layout.spec.ts`
  - 预期：表单不出现手填用户 ID；空态文案与“未启用团队”语义一致。失败复查：对照服务端 `backend_capabilities`/治理可用性响应，确认空态分支判断字段未漂移。

## 6. 全量回归与证据

- [x] 6.1 全量单测/组件测试与构建。
  - owner: client；范围：全项目。依赖：第 1–5 组全部完成。
  - 验证：`pnpm test`；`pnpm build`
  - 预期：全部通过且构建产物无类型错误。失败复查：先定位失败文件所属分组，按该组验证命令单跑复现；脏工作区并行改动导致的失败需归因后再修，不得顺手改无关逻辑。
- [x] 6.2 全量 e2e 与既有文件流程回归（Finder、Media、History、Sharing、LocalSharing）。
  - owner: client；范围：`tests/*.spec.ts` 全部。依赖：6.1。
  - 验证：`pnpm test:e2e`
  - 预期：既有 spec（`tests/finder.spec.ts`、`tests/finder-actions.spec.ts`、`tests/demo.spec.ts`、`tests/workspace-access.spec.ts` 等）无回归。
- [x] 6.3 集成与浏览器验收证据写入 `temp/integration-test-runs/<run-id>/`（`summary.json`、`command.txt`、`stdout.log`、`stderr.log`、`env.json`、`artifacts/`），失败同样留存并以原退出码结束。
  - owner: client；范围：`temp/integration-test-runs/`。依赖：6.1、6.2。
  - 验证：由 repository evidence runner 执行，任务状态由该命令写入。
  - 预期：证据齐全且不含凭据、token、Authorization 头。失败复查：补齐缺失文件后重跑 runner，不在证据目录外另存结果。

验证证据：`temp/integration-test-runs/20260916T164311Z-91bbb935/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
