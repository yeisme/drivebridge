# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。

- [x] 1.1 修正当前开发文档、MCP 校验和 active OpenSpec 路径，保留原任务状态与证据 | evidence: verification.md; current paths and task state preserved
- [x] 1.2 安全移除旧软链接与本地子模块注册，保留统一仓库和历史备份 | evidence: temp/integration-test-runs/20260930T154545Z-432404ee; idempotent cleanup and fail-closed preflight checked
- [x] 1.3 完成入口、MCP 定向校验、普通编排测试和规格验收后归档 | evidence: verification.md; workspace 5 passed; MCP static checks passed; OpenSpec 78 passed
