# DriveBridge

本项目已按用户明确要求统一为 `cli/drivebridge` 一个 Git/文档/OpenSpec owner。根模块维护 CLI/MCP，`server/` 是 HTTP/GORM 服务模块，`web/` 是 React/TypeScript/Vite 模块；模块可独立构建，但不再分别维护 Git 或 active OpenSpec。前端仅消费 API，文件/版本/授权仍由服务层裁决。

日常命令：`make dev`（仅启动可丢弃本地 fixture 与 Vite）、`make build`、`make test`、`make e2e`、`make check`、`make specs`。统一证据位于本项目 `temp/integration-test-runs/`。旧工作区入口已按用户要求移除，新工作仅从此目录开始。产品 PRD 见 `docs/web/product/personal-agent-workspace-prd.md`，开发入口见 `docs/development.md`。

OpenSpec 的 F/B 任务统一使用同仓 `python3 scripts/openspec-tasks.py` 维护，保留任务状态与历史证据。任务中 web 的 src/tests 路径相对 `web/`，server 的 internal/sdk/cmd 路径相对 `server/`；日常全项目命令在根目录，模块定向命令在相应模块执行。

## Electron 与 Pinax 阅读设计准入（2026-10-01）

用户指定 Electron，三平台实机验收，阅读先行。规划 `desktop/` 只拥有 main/preload/Forge，复用 `web/`，Go application service 保持领域真源及 `CGO_ENABLED=0`。Pinax 通过公开合同接入，不直接读 `.pinax/**` 或笔记数据库；本机阅读不要求云登录，不复建独立 Reader/Workbench。受管 vault 的所有 DriveBridge mutation 入口由应用层保护，renderer 不持凭据、不执行任意 shell/fs。实现依据为 `openspec/changes/drivebridge-desktop-knowledge-reader-v1/` 与 `docs/architecture/electron-desktop.md`；目前只交设计与任务，未新增产品实现或替代原 Web/PWA/移动端需求。历史“不做 Electron”不再约束该已批准宿主。

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


CLI/MCP 模块负责 OneDrive 空间、文件引用、受控传输和短期交接；Server 提供个人文件平台，Web 提供用户界面。生成准入、Provider 凭据和领域任务状态仍属于创作 owner。接口为 pre-1.0 增量合同。

- 采用 Go、固定版本 rclone、MCP SDK；持久化使用 GORM 和 pure-Go SQLite，支持 `CGO_ENABLED=0`。
- 所有本机路径限制在配置的 roots；远端访问限制在配置的 drive/root。配置和回执只通过 CLI/application service 写入。
- 不在 MCP 文本、日志、回执或证据中输出 token、临时 URL、原始请求。禁止公共分享、永久删除（软删除+恢复仅限空间内）、默认覆盖、自动付费重试和真实账号操作。
- 新入口默认关闭。独立本地 Git 仓库；远程创建、发布及部署单独授权。
- 使用 `go test ./...`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`；集成验证使用 `python3 scripts/check.py`，证据位于 `temp/integration-test-runs/`。
- 使用 owner OpenSpec 记录实现和验收；普通文档中文，命令与错误英文。根工作区中其他项目的改动必须保留。

## 本域 Skills

Active skills 由仓库根 `.skills/profiles/targets/cli/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target cli/drivebridge` 生成运行副本。对照见 [子项目 Skill Profile](../../docs/skills/subproject-skill-profiles.md)。

- 实现、调试、测试：`yeisme-coding-execution-driver`。
- MCP tool/schema/transport：`yeisme-mcp-builder`。
- 本机 OneDrive/引用/传输：`drivebridge-operator`。生成任务仍归创作 owner。
- Go CLI/后端：`golang-cobra-viper-cli-architecture`、`backend-system-workflow`。
- 完成前：`review`、`health`、`cso`。
