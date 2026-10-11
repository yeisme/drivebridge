## 1. 实施与验证

- [x] 1.1 owner=cli/drivebridge；module=server；合同基线；依赖前置包合同；核对本文源码基线、旧接口与批次与原幂等身份；验证 openspec validate drivebridge-upload-batch-recovery-v2 --strict --no-interactive；期望设计与旧行为一致；失败重查基线，不改旧任务。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [x] 1.2 owner=cli/drivebridge；module=server；合同基线；依赖1.1；固定design接口表的输入/响应/错误/权限/幂等/分页及SDK映射；验证同上strict与合同样例人工复核；期望无当前可用性冒称；失败回1.1。设计证据：本包design/spec及定向strict输出；不代替运行证据。

## 2. 实施与验证

- [x] 2.1 owner=cli/drivebridge；module=server；模型与迁移；依赖1.2；先新增本包TestFPUploadRecovery模型/合同测试入口（Client为同主题Vitest suite），随后同步实现；实现design数据段的GORM状态/引用模型与唯一键，保持旧ID和URI；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1；期望固定身份、隔离及幂等反例通过；失败保留证据并修本包模型。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 2.2 owner=cli/drivebridge；module=server；模型与迁移；依赖2.1；实现加法schema/backfill、旧数据读取与回滚边界；独立可重建投影不成为真源；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1；期望旧fixture继续可读，迁移不丢引用；失败回2.1，不删旧列。证据：本owner temp/integration-test-runs/<run-id>/六件套。

## 3. 实施与验证

- [x] 3.1 owner=cli/drivebridge；module=server；应用服务；依赖2.2；实现batch分页枚举/seal、原键original_request lookup、每客户端片独立耐久暂存ACK与后端分片重组，扩展2.1已建立TestFPUploadRecovery入口；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1；期望创建响应丢失可恢复且顺序8MiB客户端不因等待64MiB而死锁；失败保留temp/integration-test-runs/<run-id>/六件套并修本owner，不换键。
- [x] 3.2 owner=cli/drivebridge；module=server；应用服务；依赖3.1；实现本包source/permission/revision变化、partial、unknown、取消与恢复；重点协议兼容与后端重组；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPUploadRecovery -count=1；期望反例保留原身份且无越权；失败回读原操作，不换键重试。复杂并发/恢复条件在代码中用英文注释说明，中文设计解释证据边界。证据：本owner temp/integration-test-runs/<run-id>/六件套。
  （-race 变体未执行，AGENTS.md race 禁令高于任务文本，deputy 无权授权 race；非 race 验证 run：20260925T093555Z-ceedf706 与 20260925T193720Z-b9d7a613）
- [x] 4.1 owner=cli/drivebridge；module=server；消费入口；依赖3.2；接本文CLI/MCP/HTTP/SDK同一应用合同；无权限的管理动作不向Agent注册；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1；期望入口结果同义，旧消费者兼容；失败修适配而非复制领域逻辑。证据：本owner temp/integration-test-runs/<run-id>/六件套。
- [x] 4.2 owner=cli/drivebridge；module=server；消费入口；依赖4.1及handoff对应消费者稳定合同；用fixture完成“文件夹上传→创建响应丢失→重选文件→原任务校验完成”；验证 python3 scripts/check.py -- go test -race ./internal/service -run TestFPUploadRecovery -count=1；期望跨owner仅用公开接口、原ID可回读；失败区分owner缺口与环境，不操作真实服务。证据：本owner temp/integration-test-runs/<run-id>/六件套。
  （-race 变体未执行，AGENTS.md race 禁令高于任务文本，deputy 无权授权 race；非 race 验证 run：20260925T093555Z-ceedf706 与 20260925T193720Z-b9d7a613）

## 5. 实施与验证

- [x] 5.2 owner=cli/drivebridge；module=server；验证；依赖5.1；10000项包含0字节/空目录/Unicode/冲突；100GiB真实流断点恢复与读回摘要，记录4文件并发时单传输buffer≤256MiB及暂存占用；不能用size字段或稀疏文件代替传输。；复用本owner现有runner，验证入口 python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1（本任务扩展规模fixture，先验证测试实际命中）；期望按上述指标报告且六件套保留temp/integration-test-runs/<run-id>/；长测先补现有runner的显式timeout/日志保留，不新建框架；失败记录未达标，不勾选。 | evidence: 2026-10-11 全指标达标：10000项批量（0字节/空目录/Unicode/冲突、分页、断点）full 档 20261010T180106Z-3f958803；100GiB 真实流 TestFPUploadRecoveryScaleHundredGiBStream（DRIVEBRIDGE_FP_SCALE=full + DRIVEBRIDGE_FP_HUNDRED_GIB=1，六件套 20261011T065640Z-2d475f19）：12,800×8MiB 部件、40% 中断后原会话恢复仅重发1部件、读回流式 SHA-256 一致、4文件并发完成、单传输峰值堆 8.9MiB≤256MiB、暂存占用 mid 42.96GB/final 107.4GB 记录于 fp-upload-hundred-gib.json；无稀疏文件/size 字段替代；新增 fixtures 落盘流式对象存储（默认关闭）；check.py 支持显式 --timeout。

- [x] 5.1 owner=cli/drivebridge；module=server；验证；依赖4.2；执行spec全部Scenario、旧客户端、并发撤权与错误注入，复用本owner runner；验证 python3 scripts/check.py -- go test ./internal/service -run TestFPUploadRecovery -count=1 和 python3 scripts/check.py -- go test -race ./internal/service -run TestFPUploadRecovery -count=1；期望非空测试及六件套位于temp/integration-test-runs/<run-id>/；失败保留日志，不勾选。
  （-race 变体未执行，AGENTS.md race 禁令高于任务文本，deputy 无权授权 race；非 race 验证 run：20260925T093555Z-ceedf706 与 20260925T193720Z-b9d7a613）
- [x] 6.1 owner=cli/drivebridge；module=server；文档与回滚；依赖5.1；更新本owner入口文档、能力状态、真实命令与回滚演练；验证 openspec validate drivebridge-upload-batch-recovery-v2 --strict --no-interactive 和 git diff --check；期望没有死链、没有把计划命令写成已上线；失败修文档。设计证据：本包design/spec及定向strict输出；不代替运行证据。
- [ ] 6.2 owner=cli/drivebridge；module=server；文档与回滚；依赖5.2/6.1及稳定代码；执行design最终门并核对证据分层；期望本地验证、真实后端、发布部署分别报告；真实身份/存储/模型未获授权或未就绪时相关验收保持未完成；失败保留原任务，不以strict代替用户闭环。
