# 规格与文档验证记录

本记录只说明本次规格交付，不证明产品实现、性能、真实云、模型或部署可用。

## 已执行

- 19 个 change 的定向 `openspec validate <change-id> --strict --no-interactive` 全部通过。
- 18 个 owner 包共 216 项实施任务，按六组组织，全部保持未勾选；根设计任务与实现完成状态分开。
- 77 条 Requirement、154 个 Scenario；检查每包的接口样例、错误恢复、依赖、Mermaid、owner 与验证入口。
- 新增及相关文档的本地链接、代码围栏、空白检查和各仓定向 `git diff --check` 已执行；新增能力包依赖图无环。
- 20 张 Mermaid 图通过已有 Google Chrome 渲染。初次运行 mermaid-cli 所需的默认 headless-shell 缺失，随后指定本机已有浏览器完成；没有安装或升级浏览器。

规格与链接检查证据：[summary](../../../../../temp/integration-test-runs/20260924T174328Z-drivebridge-spec-docs/summary.json)、[检查日志](../../../../../temp/integration-test-runs/20260924T174328Z-drivebridge-spec-docs/stdout.log)。图表产物：[渲染导航](../../../../../temp/integration-test-runs/drivebridge-spec-diagrams/artifacts/rendered.md)。这些是本工作区诊断产物，独立 checkout 可重新执行定向检查。

## 既有问题与并发工作

根 `docs/README.md` 有六处既有失效链接，涉及已退役执行 owner、Auctra、Pinax 挂载、个人数字人和 structured judgment 的旧 change 地址。去掉本轮唯一新增的导航条目后，文件 SHA-256 与修改前基线完全相同，确认断链不是本轮引入。详见证据 summary 的 `baseline_issues`；没有为通过本轮检查重写无关历史索引。

工作期间根 `identity-platform-removal-v1` 被其他工作归档；本轮未写这些文件，也没有恢复或覆盖。既有 Manju、DriveBridge 和客户端 OpenSpec 任务保持原哈希；本轮只生成自己的新任务包，并定点修改列明的产品文档。

## 未执行

产品代码测试、PostgreSQL 迁移、ClickHouse/DuckDB 运行、百万文件/100GiB压力测试、真实存储/OAuth、模型/费用调用、跨机器部署和发布均未执行。相应任务继续保持未完成；规格通过不改变运行能力声明。
