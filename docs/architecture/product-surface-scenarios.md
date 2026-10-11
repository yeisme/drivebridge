# 产品面与交付范围补充

本页补齐个人工作台 PRD 之外的本机与远程使用路径；不替代 [R01–R15 能力账本](../web/product/personal-agent-workspace-prd.md) 或 [AC01–AC30 验收矩阵](../web/ui/personal-workspace-acceptance.md)。本页是设计说明，不维护执行状态。

## 首期结论

采用“个人文件平台 + 多 Agent 受控协作”的既有定位。第一条新增落地闭环仍为 S0 基础保护与合同、S1 文件上传/刷新恢复/固定版本、S2 受限 Agent 交回产物和人类归档。S2 不等待知识库、资源库全部实现，但最小收件链不替代完整 S2 的组织、期限、计划和标签建议验收。

S3 承接知识与资源，S4 承接 Web 能力完整性和既有兼容回归；最终 Web 版本覆盖 S0–S4 中适用于 Web 的必需能力。S5 保留内置助手、原生离线/相册/后台能力，待独立合同和宿主实测后启用，不以 Web 模拟实现顶替。

CLI/MCP 本机能力不是等待新版 Web 才可存在的附属品：保留其独立使用路径和本机安全边界。统一仓库并不要求同一时刻启动三个模块，也不要求将本机私有状态直接搬入远程服务。

## 跨产品面场景矩阵

| 场景 | 用户与目标 | 准入 / 权威边界 | 必需产物与验收门 | 交接与证据路径 | 阶段与成熟度 |
| --- | --- | --- | --- | --- | --- |
| 本机 OneDrive 文件往返 | 本机用户或已授权 Agent 查找、上传、读取固定版本 | fit；根 CLI/MCP 持有配置空间、local roots、引用与本机操作 | 固定引用、原 operation；拒绝路径越界、版本漂移、默认覆盖 | 根 CLI/MCP 回执；`temp/integration-test-runs/<run-id>/` | 既有源码/历史证据待当前复核；S0/S4 保护，真实云独立验 |
| 通用存储文件往返 | 用户经已有 rclone 连接使用 S3/WebDAV 等 | fit；既有 adapter 和能力发现，不推断每个 provider 功能等价 | list/stat/transfer 与明确 unsupported；不得将 generic 宣称有 OneDrive 临时交接 | 固定 file ref 和原操作；同 owner 证据目录 | 既有能力逐连接验收；不扩展为本轮全云实测 |
| 本机同步与冲突恢复 | 用户在本人终端预览再执行同步 | fit；本机 SyncPlan/基线与单 writer；MCP 仅发现已有只读能力 | 扫描覆盖、plan revision、两端根和摘要；不完整扫描不传播删除；未知先回查 | 原计划与 operation；见 local-file-experience | 既有文档与源码能力待复核；新 Web 不新增隐式 apply |
| 远程个人空间 | 用户在浏览器浏览、上传、预览、管理文件 | fit；server 应用服务为该空间数据和权限真源，web 仅投影 | Session/CSRF、File/Version/Operation；上传发布与发送进度分开 | 固定版本、HTTP+SQLite 回读；AC04–07 | S1；first-support 目标，未以原型证明 |
| 未安装 CLI 的 MCP 客户端 | 外部客户端发现能力并在授权目录读写 | fit；已连接 MCP surface 的实际 schema 与资源；字节走受权传输合同 | tools/list/inputSchema、原请求身份与 operation；不将客户端路径当远端路径 | 已注册工具返回的引用与回执；SDK/MCP 负例证据 | S2 合同与集成；禁止虚构 upload 工具或 URL |
| 多 Agent 产物回存 | 人类分别授权多个 Agent、接收和归档其产物 | fit；server 管 grant、staging、文件决定；外部 Agent 自行发起调用 | 仅目录内动作、真实主体、固定版本、幂等归档；Agent 不自批 | invocation、submission、原 operation；AC12–17/24–30 | S2；first-support 目标 |
| 创作资源交接 | 创作者向 Scaena/Eikona/Sonora 提供参考并保存产物 | split-owner；DriveBridge 文件与传输，创作 owner 管生成、费用和领域采纳 | 受权固定引用、传输回执、领域任务引用；文件接收不等于生成审核通过 | 各 owner 公开合同，DriveBridge 不保存 provider 原始载荷 | 保留既有能力；跨机/真实模型另验，不在本目标触发 |
| 知识与资源工作台 | 用户浏览资料、Markdown、模板和资料包 | split-owner；DriveBridge 管普通文件/版本与安全投影；Pinax 等管各自结构化资产 | base_version 冲突、固定资源检查结果；存储不等于安装或执行 | 文件版本及 owner 操作入口；AC08–11 | S3；first-support 目标 |
| 团队与治理兼容 | 既有管理员继续访问合法管理入口 | fit；既有 server 角色、tenant 和授权合同 | 旧 URL/API 与隔离负例；个人模式不扩权、不删管理能力 | 原角色与资源合同；AC22–23 | S0 保护、S4 回归；不要求个人首条闭环部署 PostgreSQL |
| 原生离线与内置助手 | 用户需要 Web 之外的后台能力或内置协作 | split-owner；未来原生宿主/批准的 runtime；文件仍归 DriveBridge | 宿主 capability、预算/授权和任务引用；缺能力明确 unsupported | 原生/runtime 实测后交接；AC20–21 | S5 exploratory 保留，不选择未经批准的新 owner |
| 通用 Agent 执行平台、公开永久分享或自动永久删除 | 超出当前交付边界的动作 | reject-now；不新增通用执行器，不放宽 owner 禁令 | UI 禁用/不提供执行入口；历史功能说明不构成本次授权 | 无执行；如后续需要，独立提出范围与授权决定 | 本轮排除；不以此删除已有历史数据或源码 |

成熟度描述是设计目标及本轮证据范围，不把历史归档、源码存在、UI 可点或单个 fixture 视为当前生产就绪。

## 核心流程与失败后的去向

- **无 Agent 的个人使用**：登录 → 选择目录 → 上传 → 服务端确认发布 → 刷新 → 打开固定版本。中断保留原请求身份，先 lookup，再恢复未完成部分；不重新创建重复文件。
- **受限 Agent 回存**：人类选目录和动作 → 签发独立 grant → 外部客户端发现合同并调用 → 受限 staging → 固定版本预览 → 人类接收 → 幂等归档。调用记录、上传成功、文件验收与领域采纳分别呈现。
- **本机与远程交接**：本机读取受限 local root → 通过明确远程目标的传输合同复制 → 远程版本确认 → 返回引用。本机路径不出现在远程“可直接读取”的承诺中，源副本不会因复制成功自动删除。
- **计划与同步**：只读扫描/预览 → 人类确认 revision 与影响 → 原 owner 执行 → 逐项回读。上下文或授权改变使旧确认失效；取消不等于回滚，补偿另建计划。

## 跨产品面完整追踪

以下每行对应上方同名场景，补充 R01–R15 之外的 CLI/MCP 路径。终端/MCP 场景以命令/协议流程作为可审阅原型，不强造 Web 页面。所有命令从 cli/drivebridge 执行，均是**后续验收方法，本轮未运行业务测试**；新增直连用例在 B12.2 创建后必须确认非零命中。

| 场景 | 设计位置 | 原型页面或流程 | 明确落地任务 | 验收方法与预期 |
| --- | --- | --- | --- | --- |
| 本机 OneDrive 文件往返 | unified-runtime-design §1/4；local-file-experience | CLI upload → operation_get → stat/download；Web本机view保留 | F01.1保护清单、F16.2入口、F24保护回归 | `python3 scripts/check.py -- go test ./internal/bridge -run 'TestUploadResumeAfterLostAcknowledgement|TestIsolationVersionAndNonOverwrite' -count=1`；fixture原ID恢复、版本固定和不覆盖；真实OneDrive另验 |
| 通用存储文件往返 | unified-runtime-design §1/3.2；storage-recipes | storage kinds → 发现能力 → list/transfer → 原操作 | F24；F13.1连接展示 | `python3 scripts/check.py -- go test ./internal/bridge -run 'TestGenericSpaceRoundtrip|TestGenericSpaceRootPath|TestGenericHandoffGated|TestSymlinkPolicy' -count=1`；根隔离、往返及unsupported语义保持 |
| 本机同步与冲突恢复 | unified-runtime-design §5；local-file-experience同步计划 | sync plan → status → 人类apply；MCP只读status | F24；不新增Web隐式apply | `python3 scripts/check.py -- go test ./internal/bridge -run TestFPSyncPlan -count=1`；扫描不完整拒删、revision/单writer保护、原操作恢复 |
| 远程个人空间 | PRD R02、UI Spec文件与预览、统一架构§4 | 冻结文件页 → 上传 → 固定预览；异常见design-review-guide | F04–F06、F19、F17.1 | AC04–07；F19真实HTTP+SQLite回读，原操作不重复，后续稳定全量Playwright |
| 未安装CLI的MCP客户端 | unified-runtime-design §3.1 | /mcp发现 → 新grant HTTP inbox-submissions → HTTP分片 → complete → submission → 人类归档；files.read另授；旧write才用create_upload | B01合同、B05强制权限、**B12.2 S2直连闭环**；B12.1 S4兼容 | B12.2真实Streamable HTTP+专用HTTP收件；旧create_upload对新grant拒绝、lookup原submission、received_parts恢复、两Agent/撤权/无local_path；B16/F23补HTTP标签建议，不等待S4 |
| 多Agent产物回存 | PRD R07/R08/R14；UI Spec §3.2/3.3 | Agent → 接入 → 收件 → 人类决定 → 归档 | B04–B08/B15/B16、F19/F23 | AC12–17/24–30；最小链与完整S2分开，真实DB回读 |
| 创作资源交接 | unified-runtime-design §5；creative-resource-relay | 固定引用 → relay原输入 → operation_get；创作面板只交接 | F24保护、F16.2入口；外部领域另验 | `python3 scripts/check.py -- go test ./internal/bridge -run 'TestRelayCompletesEachOwnerOriginalInput|TestRelaySourceAndTargetBoundaries|TestRelaySourceChangeBlocksRecovery|TestScaenaOutputUsesOnlyExistingTaskAndPrivateGrant' -count=1`；本地owner替身无新生成、原输入/撤销/版本边界，真实模型不计通过 |
| 知识与资源工作台 | PRD R03–R06；UI Spec知识/资源 | knowledge/resources/collections/search冻结页 | B09/B10、F07–F09、F17.1 | AC08–11；版本冲突保草稿、受保护结构化资产不任意写、保存不执行 |
| 团队与治理兼容 | PRD R13；统一架构§1/7 | settings → 受权旧深链 | F01/F16.1、B12.1、F17.1 | AC22–23；角色/tenant负例，个人模式不扩权；PostgreSQL部署独立验证 |
| 原生离线与内置助手 | PRD R11/R12；UI Spec设备/助手 | devices unsupported、上下文助手未就绪 | B14/F18；F15验证Web能力降级 | AC20–21；Web无假在线/后台；S5另定宿主/runtime真实实测 |
| 通用执行平台/永久公开分享/自动永久删除 | owner禁令；统一架构§7 | capabilities禁用原因；回收站不启用危险动作 | F21.2、B11、F17.1 | 禁止入口和服务端拒绝断言；不运行危险外部写、不借历史功能说明扩权 |

F24 是同仓现有 CLI/MCP 能力的保护验收，不复制领域实现；F17.3 依赖 F24 才能完成最终 Web 交接。以上跨产品面方法与既有 AC 表并列覆盖，不把 S0/S4 阶段标签冒充任务。

## 当前决定与后续门

核心 Web 首期范围、同仓 owner、SQLite 优先及既有前端栈都有现有明确依据，不需要重复询问。当前不引入新的商业定位、付费模式或市场结论，因此本阶段不以未经研究的竞品判断改变范围。

原生宿主框架、内置 Agent runtime 的具体接入、真实云账户与生产部署仍是对应后续切片的决策门，不阻塞本次 Web 设计包。若用户希望它们进入当前首期，应先调整目标范围，再补具体设计，不能悄悄降级成已完成。
