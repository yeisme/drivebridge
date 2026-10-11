## Why

唯一 owner 为 `cli/drivebridge`，CLI/MCP、server、web 是同仓模块。源码 src/tests 相对 web/；OpenSpec、任务脚本和统一证据均在仓库根，运行位置见 [开发入口](../../../docs/development.md)。

当前线上原型已改为个人空间、多 Agent、统一导航与可调整详情；现有生产客户端仍有大入口、多轮 Finder/CSS 和旧总览/四中心页面。用户要求正式拆分前后端规格，先清理旧前端并重建，避免继续叠加页面分支。

## What Changes

- 准入 fit；同仓 web 展示，server 拥有文件、版本、权限、组织、计划和回执；Pinax/Manju 等外部领域为 split-owner。PRD R01–R15 全部保留，按 D0、S0–S5 分层；S2 Agent 先于 S3 信息工作台。
- 用当前原型及补齐的 UI 状态驱动新的壳层、文件/知识/资源、Agent/任务/收件、存储与管理页面。
- 拆旧 main/Finder 页面编排和冲突样式；保留安全 reader、API/CSRF、恢复身份、固定引用与旧管理/分享/本机合同。
- 形成逐页控件、响应式、错误恢复、路由兼容、数据隔离和双端验收标准。
- 补齐长期/六档限时、分组置顶、卡片/进度与 MCP/API 接入；不保留演示执行或新建外部任务执行器。用户已选择最新冻结快照，本轮v39；历史v36仍未核实但不再阻塞，摘要见reference。新增分组展开/收起与任务筛选偏好仅UI，不改授权。
- 固定技术选型：现有React/TS/Vite、hash路由、局部reducer/typed fetch、安全阅读器；S2补建议审阅F22与完整门F23，真实联调从单端任务分离。
- 本次交付是文档和规格；实施任务保持未完成，原型和产品源码不在本轮改写。

## Capabilities

### New Capabilities
- `personal-workspace-shell`：清理旧壳、新视觉、响应式、兼容与偏好。
- `personal-file-workspace`：文件预览、知识资源组织检索及管理页面的统一交互。
- `personal-agent-experience`：独立授权、计划恢复、收件和可观察结果。

### Modified Capabilities
既有合同和已归档规格不删除；布局重建保持公共行为，迁移映射在 design 与重建文档。旧深链至少保留一个实际发布周期，未明确移除版本前持续支持。

## Impact

前端实现目录为本 owner web/src、web/tests 与必要脚本/docs；本轮只改设计文档。新合同依赖 Server `drivebridge-personal-workspace-contracts-v1`；原有 information-workspace-v2/file-platform-consumers-v2 的应用语义复用。不会清空浏览器恢复库、用户数据或服务端状态。原生宿主与内置助手保留后续任务，不虚构已实现。
