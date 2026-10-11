# 新版个人网盘与 Agent 工作台：前端开工包

唯一 owner 为 `cli/drivebridge`。web 源码路径相对 `web/`，任务工具、OpenSpec 校验及统一证据在仓库根。默认同 checkout、独立端口和 disposable 数据，见 [开发入口](../../../docs/development.md)。

状态（2026-10-08）：首次完成申请被独立审计否决；C110–V114的收件可读信息、非文本固定版本预览/下载及稳定态全站/详情/弹层矩阵已实现并有本地证据，随后独立审计已批准本轮限定Web与核心本地真实闭环。I109仅保留审计前分层证据，不能替代返工验收。当前6条真实本地业务/布局场景为20261008T140404Z-b6b40e0d，稳定204观察为140335Z-8cebb779，完整对照图集为140543Z-2c1dd519；详细失败归因及限定范围见design。当前参考和逐页结果见 [2026-10-07 冻结基线及10-08交付补充](../../../docs/web/ui/workspace-visual-gap-baseline.md)；[v39](../../../docs/web/ui/personal-workspace-reference.md)仅为历史记录。F87–F99完成页面分片，V100/V101为服务验证，I102/I103为首尾链，B104–I108为自动化安全及管理合同。2026-10-09按用户选择仅归档[F87–V114共28项已验收切片](../archive/2026-10-09-drivebridge-web-workspace-accepted-slice-v1/design.md)，原编号、完整证据与拆分前快照均保留。原任务位置改为归档索引，其余164项任务原文和状态不变（123完成、41未完成）。本包仍含Pinax/原生/更大产品任务，保持active；原delta specs不随本次记录整理同步或归档。

- [proposal](proposal.md) / [design](design.md) / [tasks](tasks.md)
- [历史设计校验与开工交接](verification.md)：仅该次 strict、链接/依赖和图表证据；当前代码结果以 tasks/design 的分片证据为准。
- [PRD](../../../docs/web/product/personal-agent-workspace-prd.md)
- [UI 规格](../../../docs/web/ui/personal-workspace-ui-spec.md)
- [技术选型](../../../docs/architecture/personal-workspace-technical-decisions.md)：SQLite单机优先、现有栈、生成合同与轮询恢复
- [清理/路由兼容/真实命令](../../../docs/web/ui/personal-workspace-rebuild.md)
- [验收矩阵](../../../docs/web/ui/personal-workspace-acceptance.md)
- [多笔记工作台正式实现规格](../../../docs/web/ui/notebook-workspace-production-spec.md) / [实现交接](../../../docs/delivery/notebook-workspace-implementation-handoff.md)
- [服务端配套包](../drivebridge-personal-workspace-contracts-v1/README.md)

下列为历史分期顺序，不代表这些入口当前仍未实现：F01–F03负责基线与新壳；S1 文件后 S2 Agent：F19 是不等待知识/资源的首条联调，F20 补分组置顶；S3 信息工作台、S4 全 Web 再用 F17 最终验收。F21补首页与保留页面；F22承接S2标签建议审阅，F23是组织/建议/计划/期限等完整S2集成门；F18保留助手/原生。本包不执行生产删除、不提交推送、不修改 OpenDesign。

```bash
openspec validate drivebridge-personal-workspace-rebuild-v1 --strict --no-interactive
```
