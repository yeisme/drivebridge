# DriveBridge Client

此目录是 `cli/drivebridge/web` 模块，属于父目录唯一 Git/文档/OpenSpec owner。新工作先读 `../AGENTS.md`；统一开发命令在父目录，本文保留网页约束。服务端归同仓 `../server`，CLI/MCP 归父目录，文档在 `../docs/web`；`openspec` 为统一规格的兼容链接。

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


个人文件与统一存储可视化 client owner（服务端网页 + 本机 `drivebridge view` 资产，人用；Agent 的 CLI/MCP 面归 cli/drivebridge）。React/TypeScript/Vite；Vitest、Testing Library、Playwright。仅消费同仓 server 模块的公共 API 与本机 view 投影，不持有数据库、Provider 规则或长期连接凭据。存储后端能力标签对齐 `backend_capabilities` 矩阵，未识别类型诚实降级。

## 本域 Skills

Active skills 由仓库根 `.skills/profiles/targets/cli/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target cli/drivebridge` 生成运行副本。对照见 [子项目 Skill Profile](../../../docs/skills/subproject-skill-profiles.md)。

- 实现、调试、测试：`yeisme-coding-execution-driver`。
- 网页 UI：`yeisme-frontend-design-router` → `ui-spec-frontend-workflow`；动效与质量门用 `yeisme-ui-motion-quality`、`yeisme-frontend-quality-workflow`。
- 文件/传输/连接语义：`drivebridge-operator`；canonical 存储与权限仍在同仓 `server/` 模块。
- 完成前：`review`、`health`；登录/密钥面加 `cso`。

## 边界与验证

- 用户登录由 OIDC 提供，浏览器会话由服务端提供；不存储登录或 Agent token 到 localStorage。
- 必须覆盖拖拽、键盘、移动端、进度、失败恢复、权限/连接失效、Agent token 一次显示与本人登录 Key 再次取回。
- 配置/测试元数据通过 package manager、CLI 或脚本生成。证据位于本项目 temp/integration-test-runs/。
- 验证：`pnpm test`、`pnpm build`、`pnpm test:e2e`。禁止真实账号授权、发布、部署和付费调用。
- 保留其他 owner 的修改。中文界面/文档，英文机器协议。
