# DriveBridge Server

此目录是 `cli/drivebridge/server` 模块，属于父目录唯一 Git/文档/OpenSpec owner。新工作先读 `../AGENTS.md`；统一开发命令在父目录，本文保留服务层约束。网页归同仓 `../web`，CLI/MCP 归父目录，文档在 `../docs/server`；`openspec` 为统一规格的兼容链接。

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


本项目拥有个人文件基础设施的 HTTP 合同、OIDC 会话、用户/Agent 权限、GORM 元数据、存储连接与传输恢复。网页属于同仓 web 模块，本机 CLI/MCP 属于同仓根模块；文件服务内的租户、team、授权与通知也归本 owner；不维护通用组织平台或生成任务。

- Go 1.26，标准库 HTTP，GORM/pure-Go SQLite，成熟 OIDC 与 AWS S3 SDK。保持 CGO_ENABLED=0 可构建。
- 每次访问校验内部 user ID；禁止请求参数决定身份、邮箱自动合并、Agent 管理连接或其他 Agent。
- 凭据和临时 URL 不进入日志/回执/MCP。凭据加密，实例密钥独立备份。没有授权不得访问真实云账号、部署或发布。
- 结构化配置、状态、OpenSpec 任务与证据由 CLI/application service 写入。
- 验证：`python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server`。真实账号验证单独报告。
- 中文文档，英文命令/错误，保留其他工作区改动。

## 本域 Skills

Active skills 由仓库根 `.skills/profiles/targets/cli/drivebridge.txt` 分配，用 `scripts/skills.sh sync-target cli/drivebridge` 生成运行副本。对照见 [子项目 Skill Profile](../../../docs/skills/subproject-skill-profiles.md)。

- 实现、调试、测试：`yeisme-coding-execution-driver`。
- HTTP/GORM/权限/传输恢复：`backend-system-workflow`、`go-rust-implementation-defaults`。
- CLI 配置与输出：`golang-cobra-viper-cli-architecture`、`ai-native-cli-output-contract`。
- 文件空间与连接语义：`drivebridge-operator`。网页与 MCP 在同仓不同模块。
- 完成前：`review`、`health`、`cso`；证据走 `project-integration-test-evidence`。
