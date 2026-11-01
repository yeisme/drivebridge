# DriveBridge 文件平台规格导航

本包只承载跨项目设计与交接。14个能力包与4个配套包的实现任务均在各 owner；本文及历史 handoff 表不作为当前实施状态真源，须查各 owner 的源码、任务与运行证据，不能据此宣称运行可用。

2026-09-30 新增开工路线：[个人多 Agent 工作台 PRD](../../../docs/web/product/personal-agent-workspace-prd.md)、[前端清理重建任务](../drivebridge-personal-workspace-rebuild-v1/README.md)、[后端工作台增量任务](../drivebridge-personal-workspace-contracts-v1/README.md)。以当前 OpenDesign 个人工作台为体验基线，保留本包文件平台和 owner 边界；不复制实现任务到根目录。

- [提案](proposal.md)
- [架构与阶段依赖](design.md)
- [能力账本和owner handoff](handoffs.md)
- [场景矩阵与验收目标](scenarios.md)
- [跨项目规格](specs/drivebridge-file-platform-owner-contract/spec.md)
- [设计任务](tasks.md)
- [规格验证与既有问题](verification.md)

验证：`openspec validate drivebridge-file-platform-evolution-v2 --strict --no-interactive`。
