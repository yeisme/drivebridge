## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与事务与投影分层；验证 openspec validate drivebridge-team-runtime-projections-v1 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPTeamRuntime模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现事务与投影分层和原任务与 fencing的正常路径；扩展2.1已建立的同主题测试入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1；期望design场景正例通过，拒绝空测试；失败修同一owner。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点迁移与代次切换；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPTeamRuntime -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 4. 实施与验证

- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“上传→重启 worker→原任务完成→投影重建→受权查询”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPTeamRuntime -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 5. 实施与验证

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPTeamRuntime -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。
- [ ] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；100万目录/1000万事件，100events/s持续15分钟，重复乱序重放与20并发读取；核对generation一致、队列无重复发布，投影P95延迟≤30秒。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。（未勾选原因：事件速率/总量当量、乱序重放、20并发读取、无重复发布与P95≤30s 已达标并留证；但 100 万目录 fixture 与 1000 万事件全集未在单机实例化，按本任务规则记录未达标：temp/integration-test-runs/20260925T080511Z-7d07c851/） | evidence: 2026-10-11 实测未达标不勾选：runner 已补显式 --timeout；TestFPTeamRuntimeScaleMillion 新增 million 档。已达标维度（200k 全管线 20261010T193750Z-9a2e469a + 20k 2026-10-11 通过）：100万目录、100events/s×15min 真实窗口、20并发读取（节拍防单连接池饿死）、lag P95=1s≤30s、乱序重放、无重复发布、generation 一致。未达标维度：1000万事件全量排水——播种已解竞争（dormant published 激活，12.7k rows/s），但 worker 生产 apply 路径随投影表增长二次方退化（890/s→170/s），4h 仅排 1.47M/10M（20261011T024035Z 系六件套失败证据），外推需 15-20h，本机不可完成；不做业务代码改动绕过。达标部分另有 10M 播种容量与 1.47M 实排进度证据。

## 6. 实施与验证

- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-team-runtime-projections-v1 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [ ] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。（未勾选原因：与 readiness 包 6.2 同一最终门证据；check.py 全量失败集=干净 HEAD 基线（既有 4 失败，非本变更引入）；vet/build/strict/git diff --check 通过；另 5.2 的目录/事件全集维度未达标见该行说明；真实后端/发布部署未获授权保持未完成。）
