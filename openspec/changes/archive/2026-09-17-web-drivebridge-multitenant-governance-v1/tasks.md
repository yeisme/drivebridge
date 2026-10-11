# Tasks: drivebridge-multitenant-governance-v1

范围：`client/drivebridge-client`（React/TypeScript/Vite）。HTTP 合同与权限权威归 `backend-server/drivebridge`；本 change 只消费服务端公共合同，不部署、不处理真实凭据、不持有长期凭据，上下文只存当前标签页内存。界面文案中文，命令、flag、路径、字段名保持英文。

完成状态说明：以下任务已由 evidence runner 执行并写入 `temp/integration-test-runs/20260916T112417Z-9dbabd40/summary.json`，统一标记为完成；任一项回归失败时取消勾选并按对应分组的失败复查步骤处理。

## 1. 租户/空间上下文与隔离（Requirement: Tenant-aware workspace）

Lane A（上下文基础设施）先行；Lane B（既有页面适配）在 1.2 完成后并行启动。

- [x] 1.1 实现标签页级租户/空间上下文 helper：上下文仅存当前标签页内存，不修改共享登录 cookie；旧部署未激活治理时保留既有视图。
  - owner: client；范围：新增上下文 helper（`src/` 下治理上下文模块）、`src/api.ts`。依赖：服务端公共合同字段确认（backend-server/drivebridge 已提供）。
- [x] 1.2 上下文版本校验：所有异步 API 响应检查上下文版本；切换租户/空间时清空旧视图、拒绝陈旧响应、取消在途传输并重新挂载工作区。
  - owner: client；范围：`src/api.ts`、上下文 helper、`src/Workspace.tsx`。依赖：1.1。
- [x] 1.3 既有 Finder/Media 小范围适配：文件请求附带租户和空间上下文，权限由服务端验证；客户端不实现权限规则。
  - owner: client；范围：`src/Finder.tsx`、`src/Media.tsx`、`src/api.ts`。依赖：1.2。
- [x] 1.4 上下文隔离单元/组件测试：切换清理、陈旧响应拒绝、在途传输取消、旧部署兼容分支。
  - owner: client；范围：`src/governance.test.tsx`、`src/Workspace.test.tsx`。依赖：1.1–1.3。
  - 验证：`pnpm test -- src/governance.test.tsx`
  - 预期：陈旧响应断言拒绝生效；取消传输后无悬挂请求错误。失败复查：确认 mock 响应携带上下文版本字段；用 `pnpm test` 全跑排除用例间共享状态。

## 2. 管理授权中心（Requirement: Approval and notification center）

Lane C（管理页结构）与 Lane D（通知/审计视图）可并行，汇合于 2.5 验收。

- [x] 2.1 新增 `src/Governance.tsx` 管理中心：覆盖团队、成员、权限、个人申请、待我审批、明确执行、应急、通知、访问记录（审计）、团队成员与数据负责人入口。
  - owner: client；范围：`src/Governance.tsx`、`src/governance.css`、`src/i18n.ts`。依赖：1.2。
- [x] 2.2 普通范围许可与精确操作清单分别呈现：清单逐项显示固定版本、动作、结果；批准后由申请人确认执行并产生执行回执。
  - owner: client；范围：`src/Governance.tsx`、`src/api.ts`。依赖：2.1。
- [x] 2.3 管理员只读状态：无写授权时可浏览、预览与下载，直接修改入口禁用，并提供进入授权中心申请的路径。
  - owner: client；范围：`src/Governance.tsx`、`src/Finder.tsx`（只读入口）、`src/Media.tsx`。依赖：2.1。
- [x] 2.4 应急访问：必须单独复核登录身份并说明原因后才放行；应急操作进入审计。
  - owner: client；范围：`src/Governance.tsx`、`src/api.ts`。依赖：2.1。
- [x] 2.5 角色按钮与治理交互测试：角色驱动按钮可见性、只读禁用、审批-确认执行流转、应急复核表单。
  - owner: client；范围：`src/governance.test.tsx`。依赖：2.1–2.4。
  - 验证：`pnpm test -- src/governance.test.tsx`；`pnpm build`
  - 预期：无写授权角色看不到修改入口；应急表单强制原因与身份复核。失败复查：对照服务端角色投影字段，确认客户端未自行推断权限。

## 3. 本人控制的账号恢复（Requirement: Owner-controlled account recovery）

Lane E 与 Lane C/D 并行（恢复流程独立页面状态），密钥用户与 OIDC 用户两条身份路径在 lane 内顺序验收。

- [x] 3.1 账号恢复统一页面：新密钥由本人生成与领取；密钥用户、OIDC 用户共用同一页面；OIDC 身份复核由服务端检查近期认证时间。
  - owner: client；范围：`src/Governance.tsx`（恢复区块）、`src/api.ts`。依赖：1.2。
- [x] 3.2 一次性显示语义：恢复码及新密钥只在一次性界面出现，关闭即清空状态；不向 localStorage 写入任何凭据；提供本人登录后再次取回 Key 的既有路径回归。
  - owner: client；范围：`src/Governance.tsx`、`src/KeyDirectory.tsx`（取回入口）。依赖：3.1。
- [x] 3.3 人工核验视图：管理员只提交恢复请求 ID，页面仅显示恢复对象与请求 ID，不显示领取证明、新密钥或恢复码。
  - owner: client；范围：`src/Governance.tsx`。依赖：3.1。
- [x] 3.4 恢复流程测试：一次性显示关闭清空、核验视图无明文、本人/管理员两种角色渲染差异。
  - owner: client；范围：`src/governance.test.tsx`。依赖：3.1–3.3。
  - 验证：`pnpm test -- src/governance.test.tsx`
  - 预期：关闭一次性界面后组件状态与 DOM 均不含恢复码/密钥；管理员视图快照无凭据字段。失败复查：检查是否有测试 fixture 误存明文；按根仓库凭据政策处理泄露输出。

## 4. 跨用户 e2e 与失效场景

- [x] 4.1 独立治理 Playwright fixture 与配置（`playwright.governance.config.ts`）：跨用户审批、只读入口、通知与审计入口可用。
  - owner: client；范围：`tests/governance.spec.ts`、`tests/admin-takeover.spec.ts`、`playwright.governance.config.ts`。依赖：第 2、3 组。
- [x] 4.2 权限/连接失效场景：会话过期、权限回收后旧操作被拒并提示重新登录；服务端不可用时治理页展示不可用状态而非假数据。
  - owner: client；范围：`src/api.ts`、`src/Governance.tsx`、`tests/workspace-access.spec.ts`。依赖：4.1。
- [x] 4.3 治理 e2e 回归：拖拽/键盘操作在只读模式下被正确禁用，移动端视口治理页可用。
  - owner: client；范围：`tests/governance.spec.ts`、`tests/admin-takeover.spec.ts`。依赖：4.1、4.2。
  - 验证：`pnpm test:e2e -- --config playwright.governance.config.ts`
  - 预期：跨用户审批全链路通过；权限回收后写操作返回服务端拒绝且界面给出恢复指引。失败复查：查看 `temp/integration-test-runs/<run-id>/artifacts/` 截图与 `stderr.log`，区分断言失败与 fixture 服务未就绪（先确认治理 mock/服务端测试实例已启动）。

## 5. 兼容、全量回归与证据

- [x] 5.1 旧部署兼容回归：未激活治理的服务端响应下，既有 Finder/Workspace 视图行为不变。
  - owner: client；范围：`src/Workspace.tsx`、`tests/workspace.spec.ts`、`tests/workspace-access.spec.ts`。依赖：1.1。
  - 验证：`pnpm test:e2e -- tests/workspace.spec.ts tests/workspace-access.spec.ts`
  - 预期：治理入口隐藏，原流程全绿。失败复查：确认能力探测字段与后端合同一致，不做客户端猜测。
- [x] 5.2 全量单测/组件测试与构建。
  - owner: client；范围：全项目。依赖：第 1–4 组全部完成。
  - 验证：`pnpm test`；`pnpm build`
  - 预期：全部通过。失败复查：按失败文件归属分组单跑复现；并行工作区引入的失败先归因再修复。
- [x] 5.3 全量 e2e 与证据落盘：默认配置与治理配置分别执行；每次集成与浏览器验收写入 `temp/integration-test-runs/<run-id>/`（`summary.json`、`command.txt`、`stdout.log`、`stderr.log`、`env.json`、`artifacts/`），失败同样留存并以原退出码结束；证据不含凭据、token、Authorization 头。
  - owner: client；范围：`temp/integration-test-runs/`。依赖：5.1、5.2。
  - 验证：`pnpm test:e2e`；`pnpm test:e2e -- --config playwright.governance.config.ts`；由 repository evidence runner 收口。
  - 预期：两套配置均通过，证据完整。失败复查：补齐证据后重跑 runner，不在证据目录外另存结果。

验证证据：`temp/integration-test-runs/20260916T112417Z-9dbabd40/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
