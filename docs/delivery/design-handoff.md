# DriveBridge 完整设计包与实施交接

本次交付是产品、架构、前端、技术选型及实施计划，不是业务代码上线。唯一 owner 为 `cli/drivebridge`；根 CLI/MCP、server、web 同仓维护，客户端不持有领域权限与数据真源。既有未提交工作全部保留，不提交、推送或部署。

## 2026-10-01 桌面阅读增量

用户已指定Electron，阅读先行且三平台实机验收。新增 [桌面阅读包](../../openspec/changes/drivebridge-desktop-knowledge-reader-v1/README.md)、[Electron架构](../architecture/electron-desktop.md)、[知识三屏](../web/ui/knowledge-reading-spec.md) 与 [独立验收](desktop-knowledge-acceptance.md)。旧S5中的桌面选型已解决；移动/后台/相册/助手仍保留S5。Pinax只作为公开合同提供方，不再新建Reader或依赖Workbench。

原74条F/B任务保持ID/状态；F06/F07/F09/B09复用，D任务仅补宿主/Pinax适配/guard/本机状态与独立阅读门。D09无需等待全部Agent或团队功能，完成它也不等于F17完整Web通过。当前仍是设计交付，不安装Electron、修改源码或运行真实vault。

## 阅读顺序

1. [现状与缺口复核](../architecture/design-baseline-review.md)：已有能力与本次证据范围。
2. [产品 PRD](../web/product/personal-agent-workspace-prd.md) 和 [跨产品面场景](../architecture/product-surface-scenarios.md)：R01–R15、个人首期和本机/远程/创作交接边界。
3. [统一运行架构](../architecture/unified-runtime-design.md)：本机与远程状态、MCP 控制与字节传输、授权、发布、恢复及备份。
4. [技术选型](../architecture/personal-workspace-technical-decisions.md)：现有栈、替代成本、迁移风险和重评条件。
5. [冻结原型审阅](../web/ui/design-review-guide.md) 与 [UI Spec](../web/ui/personal-workspace-ui-spec.md)：关键页面、状态线框、tokens、控件、双端与无障碍。
6. [API 合同](../server/personal-workspace-contracts.md)、[数据模型](../server/personal-workspace-data.md) 和 [需求追踪/验收矩阵](../web/ui/personal-workspace-acceptance.md)。
7. [前端实施任务](../../openspec/changes/drivebridge-personal-workspace-rebuild-v1/tasks.md) 与 [服务合同实施任务](../../openspec/changes/drivebridge-personal-workspace-contracts-v1/tasks.md)。任务状态由 CLI 维护，设计完成不勾选业务实施。

## 首发技术与范围结论

保留 React/TypeScript/Vite、现有 Base UI/shadcn 原语、typed fetch、页面 reducer 与 hash 路由；后端保留 Go 应用服务、GORM/pure-Go SQLite、现有存储和任务运行时。PostgreSQL 保留独立验证路径。Go DTO 生成 schema/TS/fixture 为 B01 规划，不假称已有生成器。无需重新选框架、拆微服务或引入第二套状态库。

产品首先完成个人文件闭环，再完成受限多 Agent 的外部接入与收件闭环。知识资源、分享/设备能力入口及既有团队治理分阶段保留；不因为“个人优先”删能力。原生宿主和内置助手留在 S5，不作为本轮 Web 已实现。

## 开工顺序与阶段边界

| 阶段 | 原子任务入口 | 交付结果和退出条件 | 失败时回到哪里 |
| --- | --- | --- | --- |
| S0 保护与合同 | F01.1/F01.2、B01.1 起步，后续依 tasks 推进 F02/F03、B01.2/B01.3、B02 | 旧能力保护清单、薄壳、生成合同与迁移保护；前端消费固定 fixture | 原任务 writer；先核对 dirty 基线、字段漂移和旧路径保护，不删无关改动 |
| S1 文件 | F04–F06/F13.1 与既有服务合同 | 上传→发布→刷新→固定版本预览，原请求恢复；单端完成仍是 INTEGRATION_PENDING | 文件/恢复任务；查原 operation、durable bytes、固定版本，不重造身份 |
| S2 Agent | B03–B08/B15/B16、F10–F12/F20/F22；B12.2→F19 最小链、F23 完整门 | 期限、组织、三动作、计划、建议和人工收件；两用户两 Agent 的真实隔离回读 | grant/收件/原任务层；最小链不等待 S3，也不替代完整 S2 |
| S3 信息工作台 | B09/B10、F07–F09/F21.1 | 知识/资源/集合/属性/搜索；版本冲突与权限过滤 | 对应合同和页面，不以 mock 代替持久化 |
| S4 完整 Web/兼容 | B11/B12.1/B13、F13.2–F17/F21.2 | Web 场景全覆盖，旧 API/深链/本机入口保护，稳定后全量验证 | 按 introduced/pre-existing/concurrent/environmental/ambiguous 归因，仅修责任范围 |
| S5 后续保留 | B14、F18 | 独立 runtime/宿主合同与真实实测后启用 | 未就绪明确 unsupported；不以 Web 页面冒充原生能力 |

每个实际任务只属于 CONTRACT、FRONTEND、BACKEND、INTEGRATION 之一；上表是阶段索引，不另造任务状态。可并行表示依赖许可，不授权子 agent；同路径单 writer。当前目标没有业务实施授权，不在本轮启动 S0 业务工作。

协议收敛：新 personal_workspace_v1 grant 的 inbox.submit / labels.suggest 明确走专用 HTTP，MCP capabilities 提供 schema/恢复发现，不新增 MCP 写工具。新 grant 的旧 create_upload 拒绝；只有旧 write 身份保留普通上传。收件创建原子产生 submission 与 inbox_staging operation，分片/complete共享旧协议但按grant+purpose收窄；files.read另行授权。完整字段与恢复以服务合同 §4.2.1 为准，B07/B12.2/B16及OpenSpec已同步。

审计补充：Server `/mcp` Streamable HTTP 直连的发现、分片上传、固定版本读取及恢复已纳入统一架构第 3.1 节；S2 B12.2 首次验收，S4 B12.1 仅做兼容回归。新增 F24 保护本机 OneDrive/generic/sync/relay，F17.3 依赖 F24。跨产品面完整追踪见场景文档，业务任务现为 74 项，全部保持待实施。

本轮通过 `scripts/openspec-tasks.py amend` 将统一架构的无 CLI MCP 发现/异机路径/恢复要求关联至 B12.1，将备份与回退交接关联至 B13.2，将原型审阅和异常状态关联至 F17.2，保留原 ID、依赖、未完成状态及历史内容。

## 验证命令与证据分层

从 `cli/drivebridge` 执行设计结构校验：

```bash
python3 scripts/check.py -- openspec validate drivebridge-personal-workspace-rebuild-v1 --strict --no-interactive
python3 scripts/check.py -- openspec validate drivebridge-personal-workspace-contracts-v1 --strict --no-interactive
```

实施后的产品命令按各任务与 [开发说明](../development.md) 选择；不存在的 `make contracts`/`make contracts-check` 只能在 B01 实现并登记后执行。任何聚合命令先检查不含 race；不通过运行未知命令探测是否安全。新增测试需确认非零命中，实现中只聚焦检查，稳定后跑完整门。

设计和本地快照核验不等于 UI 交互、HTTP+DB、真实云、规模或部署通过。项目 runner 的 layer 字段为通用分类，本次记录按实际工作理解为 documentation-validation / prototype-integrity，不冒称组件业务测试。

## 后续决策门与风险

当前首期、SQLite 默认与前端技术方向已有明确结论，无需再次选型。桌面Electron已确定；S5移动原生/runtime、真实云身份、生产目标和规模验收仍需在进入相应切片时另决；不以此阻塞本轮设计交接，也不把它们标完成。

历史 runbook 存在早期单租户、秘密配置与分享边界表述；后续实际部署前必须按当前代码与 owner 规则复核，不能照旧说明执行生产动作。本轮统一架构明确当前边界但不擅自变更实现或凭据。冻结原型的局部双选中与手机长页已转为正式验收要求；缺失异常交互不得靠原型截图掩盖。

最终校验结果见 [本轮设计核验](design-verification.md)。
