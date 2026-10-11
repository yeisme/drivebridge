# 个人多 Agent 工作台：服务端配套开工包

唯一 owner 为 `cli/drivebridge`。server 源码路径相对 `server/`，OpenSpec、任务工具和证据在仓库根；命令使用 go -C server 或明确进入模块，见 [开发入口](../../../docs/development.md)。

状态：合同与技术选型设计修订、业务待实施；用户选择最新冻结快照，本轮v39，历史v36不再为门禁。已有文件平台源码复用，本包只补新工作台需要的持久化与接口缺口。

- [proposal](proposal.md) / [design](design.md) / [tasks](tasks.md)
- [接口合同](../../../docs/server/personal-workspace-contracts.md)
- [数据与迁移](../../../docs/server/personal-workspace-data.md)
- [技术选型](../../../docs/architecture/personal-workspace-technical-decisions.md)：SQLite优先，PostgreSQL保留，B01生成方案与拟议命令
- [产品 PRD](../../../docs/web/product/personal-agent-workspace-prd.md)
- [前端配套包](../drivebridge-personal-workspace-rebuild-v1/README.md)

先完成 B01.1–B01.3 的 Go DTO 统一生成与漂移门、B02 增量模型；B15补个人组织、B16补S2标签建议/人工CAS；B12.2是无B09/B10依赖的最小联调，F23承接S2完整验收。前端可按冻结合同建 mock；新 grant、受限回存与最终写入都必须由同一应用服务裁决。旧 Agent、旧工具、固定文件引用和原操作不改名。

```bash
openspec validate drivebridge-personal-workspace-contracts-v1 --strict --no-interactive
```
