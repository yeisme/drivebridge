## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=web；合同基线；依赖前置包合同；核对本文源码基线、旧接口与总览与钻取同口径；验证 openspec validate drivebridge-information-workspace-v2 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [x] 1.2 owner=cli/drivebridge；module=web；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=web；模型与迁移；依赖1.2；先新增本包TestFPInformationWorkspace模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；定义Client消费类型、上下文隔离和仅界面临时状态，不建领域数据库；验证 pnpm test；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 2.2 owner=cli/drivebridge；module=web；模型与迁移；依赖2.1；补旧服务缺字段/unsupported、缓存失效与持久恢复元数据迁移；验证 pnpm test；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=web；应用服务；依赖2.2；实现总览与钻取同口径和文件详情与版本的正常路径；扩展2.1已建立的同主题测试入口；验证 pnpm test；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 3.2 owner=cli/drivebridge；module=web；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点权限和响应交互；验证 pnpm test:e2e；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=web；消费入口；依赖3.2；将本文人用入口接到真实typed client，保留旧路由并加能力门；验证 pnpm test；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 4.2 owner=cli/drivebridge；module=web；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“资源总览→筛选钻取→固定文件版本→证据和操作回读”；验证 pnpm test:e2e；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=web；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 pnpm test 和 pnpm test:e2e；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。
- [x] 5.2 owner=cli/drivebridge；module=web；验证；依赖5.1；百万目录分页fixture、万项批次；已挂载表格行≤200，320/390/768/1440px和键盘可用；20并发请求fixture只呈现当前context，图表钻取口径一致。；复用本owner现有runner，验证入口 pnpm test（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=web；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-information-workspace-v2 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [x] 6.2 owner=cli/drivebridge；module=web；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。
