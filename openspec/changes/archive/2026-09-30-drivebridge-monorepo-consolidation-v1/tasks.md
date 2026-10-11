# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。

- [x] 1.1 迁移前清单和私有 Git 备份，保全所有 dirty 文件；验证迁移脚本 plan 与 digest。 | evidence: 迁移CLI核对1847文件零摘要差异；原Git与索引在.git/consolidation-backup。
- [x] 1.2 迁移为单 Git、统一 docs/OpenSpec 与旧路径兼容；验证 file digest、任务状态、git rev-parse 和 openspec list。 | evidence: 三个模块同一Git；77项strict通过，20260930T120041Z-46f3ece9；旧任务状态保留。
- [x] 2.1 统一 dev/build/test 入口并修正跨模块路径；保留两 Go module 和 HTTP/MCP 合同；验证构建与 fixture 闭环。 | evidence: 三模块build、224项Web测试、3条浏览器流程及MCP真实fixture通过；见verification.md。
- [x] 2.2 更新 owner/README/PRD/profile 与迁移说明；验证文档路径、严格规格、普通测试和 git diff --check；禁止 race。 | evidence: profile/链接/diff通过；Server四个失败在迁移前摘要基线同样复现，未冒称全绿；见verification.md。
