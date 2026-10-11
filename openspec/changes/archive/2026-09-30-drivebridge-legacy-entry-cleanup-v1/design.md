## Context

三个旧路径已经是软链接，实际源码合并在 cli/drivebridge。此次用户清理要求取代原迁移记录中“至少保留一个发布周期”的约定；只删除入口，不递归删除目标。

## Decisions

使用 scripts/retire_workspace_entries.py 检查所有目标、原 Git 备份、profile 与 .gitmodules 后执行 unlink，通过 git config 删除旧本地注册并更新父仓索引。若发现真实目录、错误目标或丢失备份则退出，不删除。

同仓 server/web 内的 docs、openspec 与 Skills 引用仍服务于模块开发，保留。当前项目文档、根级路由和 active OpenSpec 使用新路径；历史归档、迁移脚本、原证据和 MEMORY 文件保留原文。旧任务文字通过既有任务 CLI amend 更新，状态和证据不变。

## Validation

检查三个旧入口不存在、唯一 Git owner、统一 profile/规格、Git 历史备份可读。运行 MCP conformance 的 DriveBridge 定向检查、普通 workspace 编排测试和 OpenSpec strict。不运行 race，不重跑与路径清理无关的业务全量测试。

## Rollback

如本地消费者仍需过渡，按迁移记录映射重新创建相对软链接；不要 reset/clean、覆盖新源码或恢复旧父仓整个索引。外部用户配置和部署不自动改写。
