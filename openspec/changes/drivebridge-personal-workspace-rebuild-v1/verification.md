# 设计校验与开工交接

## 2026-10-01 最新快照与技术选型交付

用户已确认“执行时读取并固定摘要的最新原型”和“SQLite单机优先”，随后明确要求执行该设计计划。本轮冻结v39，历史v36不再是D0前置，历史未核实事实不改写；未自动恢复或改写已暂停的旧持久目标。

- [v39资源快照与manifest](../../../temp/integration-test-runs/20260930T183020Z-c693eca9/artifacts/snapshot-manifest.json)：入口+12资源双读一致，浏览器全部响应来自冻结字节；19观察点无pageerror/页面横向溢出，仅证明模拟参考可读，不证明完整视觉/业务通过。
- [技术决定D01–D14](../../../docs/architecture/personal-workspace-technical-decisions.md)：现有React/TS/Vite/hash路由/reducer/fetch/net/http/GORM；Go DTO→JSON Schema/OpenAPI3.1→TS/fixture与Ajv开发校验；有界轮询与SQLite优先。拟议生成工具/依赖未实现未安装。
- 73项业务任务全部未勾选：保留既有69项编号/状态/历史，追加B16.1/B16.2/F22/F23。S2建议动作独立于S3收藏；F19/B12.2最小链不等S3，F23汇合完整Agent能力；FRONTEND仅合同fixture，本地真实链归INTEGRATION。
- [设计检查](../../../temp/integration-test-runs/20260930T184239Z-122dbe89/artifacts/design-check.json)：当次27文件/129链接、35 Requirement/83 Scenario、73任务/164依赖边，无缺失/环；检查任务状态保护、单端依赖、表格和R01–R15/AC01–30。此为交接追加前统计，最后复验以运行记录为准。
- 两份strict已通过：[rebuild](../../../temp/integration-test-runs/20260930T184239Z-4b237c73)、[contracts](../../../temp/integration-test-runs/20260930T184241Z-9b11d483)。[5张Mermaid渲染](../../../temp/integration-test-runs/20260930T184502Z-5324251b/artifacts)、[diff空白检查](../../../temp/integration-test-runs/20260930T184506Z-4d9a360b)通过。
- 本轮不改业务代码、不安装依赖、不执行产品测试/构建/数据库迁移，不接真实云/模型、不改远端原型、不部署/提交/推送、不运行race。新增的采集/校验脚本仅位于temp诊断目录，证据六件套位于仓库根temp。

设计完成不等于73项业务实现通过；后续从F01/B01按依赖开工。D0的基线选择已解决，正常业务实现仍需各任务的真实证据。以下记录均为历史轮次，不把当时“未决”当作当前门禁，也不修改历史数字。

## 2026-10-01 早先修订与当时未决项（历史）

本轮只修订设计/spec/任务和文档，未执行前端删除/重写、后端实现、合同生成器实现或真实服务变更。69 项实现任务全部未完成（原 62 项保留、新增 7 项）；旧任务编号/状态/历史证据通过 amend/append 保留。完整追溯见 [R01–R15 与 AC01–28](../../../docs/web/ui/personal-workspace-acceptance.md)。

**当时D0尚未退出（已由后续用户选择最新快照解决）：用户指定历史v36，远端实际入口为v37。** `?v=36` 返回与 v37 相同字节，不能证明历史快照；需取得可信 v36 或由用户决定改用当前基线。不因 strict 通过隐藏该缺口，也不自行改写目标。

| 核查 | 本轮事实与证据 |
| --- | --- |
| 资源基线 | [资源摘要/查询比较](../../../temp/integration-test-runs/20260930T171548Z-91a4dfbf/artifacts/asset-comparison.json)，v33 历史摘要重新核对一致；v36 不可独立验证 |
| 原型 UI | [双端首页/Agent/接入表单/搜索瞬态](../../../temp/integration-test-runs/20260930T171704Z-940832d0/artifacts)；模拟页面只读，不是业务通过。旧遍历在已移除的搜索按钮处超时，[失败记录](../../../temp/integration-test-runs/20260930T171412Z-8a2f0be1) 保留，未声称全页覆盖 |
| 两份 OpenSpec strict | [前端](../../../temp/integration-test-runs/20260930T173317Z-1b7c8fa6)、[后端](../../../temp/integration-test-runs/20260930T173319Z-fd220bb0) 均通过；仅设计结构有效 |
| git diff --check | [检查](../../../temp/integration-test-runs/20260930T173320Z-8bb6559b) 通过；大量既有 untracked 文档另外做内容/基线比较，不把 Git 空 diff 当无修改 |
| 任务与追溯检查 | [定向设计检查](../../../temp/integration-test-runs/20260930T173543Z-bda039bf/artifacts/design-check.json)：23 文件、70 本地链接、33 Requirement、76 Scenario、69 未勾选任务、136 依赖边，无缺失/环；F19/B12.2 祖先无 F07–F09/B09/B10。此统计为交接页追加前；追加后复验 [23 文件/78 链接](../../../temp/integration-test-runs/20260930T173726Z-26d4d86e/artifacts/design-check.json) 同样无问题，最新包含链接数以最后运行记录为准 |
| 本轮 Mermaid | [4 张 SVG](../../../temp/integration-test-runs/20260930T173722Z-6fe74dfb/artifacts) 已实际渲染通过，不复用旧渲染结果 |
| 业务/外部/发布 | 未运行产品测试或构建、未接云账户/模型、未改远端原型/凭据、未部署/推送、未运行 race |

修订重点：新 profile 的 never/duration 与六档期限、CAS 生命周期/轮转保期限、服务端个人组织与跨设备冲突、外部接入非执行器、Go DTO 统一生成+漂移门、逐页权限/状态恢复，以及 D0/S0–S5 分期。F01.2 改为核验现有无 race 的 runner，默认同 checkout 按独立端口/可丢弃数据隔离；新运行证据统一在根 temp。

以下为 **2026-09-30 历史交接记录**，其日期、测试计数和证据不升级为本轮验证；迁仓前路径保留，不搬动历史证据。

## 历史校验结果

| 范围 | 结果 / 证据 |
| --- | --- |
| 当前原型核对 | 只读 HTTP 资源摘要、15 个桌面入口、双端首页与 3 个详情场景；见 [reference](../../../docs/web/ui/personal-workspace-reference.md) |
| 前端 OpenSpec strict | 通过；[运行记录](../../../web/temp/integration-test-runs/20260930T090847Z-f1788f10) |
| 后端 OpenSpec strict | 新增多 Agent 建议隔离场景后复验通过；[运行记录](../../../server/temp/integration-test-runs/20260930T091359Z-3683a01a) |
| 既有根交接包 strict | `drivebridge-file-platform-evolution-v2` 定向验证通过；仅修改导航与状态说明 |
| 需求和任务 | 6 个 capability、29 条 Requirement、61 个 Scenario；37 项前端任务、25 项后端任务，全部未勾选 |
| 任务依赖 | 62 个节点、122 条依赖，无缺失引用或环；S5 外部 runtime/宿主就绪是显式阶段条件 |
| 文档检查 | 本轮 28 个文件、136 条本地链接、围栏和尾部空白检查通过；[最终内容检查](../../../web/temp/integration-test-runs/20260930T091359Z-3a2f3967)（本交接页新增前的范围） |
| 架构/状态图 | 4 张 Mermaid 实际渲染成功；[渲染产物](../../../web/temp/integration-test-runs/20260930T091029Z-ab4c76b2/artifacts) |
| 差异检查 | 前后端本轮文档路径与根交接 README 的 `git diff --check` 通过 |
| 产品测试/构建 | 本轮未运行；以上均为设计和原型读取证据，不作为业务验收 |
| 真实云/模型/原生/发布 | 本轮未执行；未修改远端原型、凭据或生产状态 |

`openspec status` 的 artifacts complete 只表示 proposal/design/specs/tasks 齐备，不表示 task 已执行。历史文件平台任务的勾选和旧原型测试计数不作为本包验收。

## 历史设计复核结论

- 当前原型与本地 v9.2 存在漂移，使用实际入口资源摘要定位，不采信画面版本徽标。
- 清理顺序明确，旧 main/Finder 页面编排进入替换；安全阅读器、CSRF、恢复身份、管理与本机/分享入口有保护清单。
- 新 grant 不映射为旧宽 write；所有调用入口和 publish 前重查限制，旧 binary 不可带新受限凭据不安全降级。
- 原标签 namespace 写入会替换整组；多 Agent 建议改为独立来源记录，人类采纳保留已有人工标签。
- 任务中心与收件使用原 operation/plan/ref；超时先回查，不重造执行身份。
- 原生和内置助手是保留需求，分阶段实施；当前 Web 未就绪状态不会伪造运行能力。

## 历史开工范围（当前依赖以 tasks/验收矩阵为准）

可以直接开始前端 F01.1/F01.2 和后端 B01.1。前端完成保护逻辑提取后执行 F02 清理、F03 新壳；后端 B01.2 生成机器合同/fixture 后，依赖它的前端页面可以开始 mock 实现。新增 API 的真实联调需相应 B 任务完成。

本轮没有生成或发布机器 OpenAPI/SDK 产物；其字段行为已由接口文档定义，生成/校验工具属于 B01.2 实施项。该依赖不能由客户端手写一份漂移协议绕过。无需再次确认普通本地开工；真实账户、费用、生产配置和发布仍按具体动作授权。
