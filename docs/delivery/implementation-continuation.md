# 实施续接记录

本页是人工阅读的工作交接，不维护机器任务状态。目标为已确认S0–S4实现，S5/部署/真实云不在范围；不运行race、不派子agent、不提交推送。当前目标里程碑基线、合同基础及后端基础完成（3/11）；下一阶段为前端壳层。下方旧进度数字是历史记录。

## 后端基础里程碑验收（2026-10-02）

按目标限定的增量模型与权限基础合同，B02.1/B02.2、跨进程迁移恢复、认证矩阵、MCP注册工具全集拒绝、应用allowOn与发布事务绑定保护已核对。当前服务层+SDK+server命令全量通过temp/integration-test-runs/20261002T062143Z-f58d825b/，pure-Go构建通过temp/integration-test-runs/20261002T062450Z-4baca4c0/。目标backend-foundation登记完成，总计3/11。业务功能完整性、前端、独立HTTP服务进程恢复、规模/PostgreSQL均不包含在此基础里程碑的完成声明中，继续由后续切片验收。

## 前端阶段基线（2026-10-02）

frontend-shell已开始；详见frontend-baseline.md。只读工作树逐文件清单证据062641Z-6463ef91，HEAD证据062641Z-309459a8；所有未跟踪/dirty条目原路径保留，未重置。生产Web Vitest全量40文件228用例通过temp/integration-test-runs/20261002T062704Z-5e417572/；定向API/路由/阅读器/存储5文件20例通过062625Z-943bceb7。当前未修改Web业务源码，F01.1仍需截图和入口核对，F01.2静态runner保护待补。总目标3/11。

## Stat基础设施错误分类（2026-10-02）

workspaceFileVersionStat首次File查询现在仅对ErrRecordNotFound返回not_found，其它失败返回database_unavailable。已有故障注入扩展到stat File/Version两查询点和缺失对照，无成功metadata投影。与固定metadata及真实HTTP/MCP读取回归通过temp/integration-test-runs/20261002T062010Z-83d103e7/，两文件无LSP诊断。

## 普通发布拒绝旧Operation快照（2026-10-02）

publishUploadAuthorized的普通分支在发布事务内复读Operation，持久化Purpose/WorkspaceGrantID任一非空即拒绝，避免只信任传入未绑定快照。测试覆盖完整/仅purpose/仅grant绑定且File/Version零写入；与撤权发布、专用complete、Blob普通回归通过temp/integration-test-runs/20261002T061718Z-accd4bdb/。两文件无LSP错误；不是全部并发/跨实例fencing验收。

## MCP注册工具全集保护（2026-10-02）

TestWorkspaceRegisteredToolSetDeniesLegacyDispatch从当前ToolDefinitions动态遍历：81个非专用工具均在受限dispatch返回forbidden，未知未来工具也默认拒绝；6个专用工具发现集合无多项/重复，capabilities正向对照成功，撤销后发现为空且对照拒绝。证据temp/integration-test-runs/20261002T061420Z-22b433cd/，测试无LSP诊断。复用真实Service fixture，但这是应用层dispatch测试，不冒充81项真实HTTP transport测试；专用工具transport已有独立证据。

## 新grant认证入口矩阵（2026-10-02）

新增TestWorkspaceAuthenticationRouteMatrix：三动作grant精确允许专用files/stat/content、suggestion、inbox、原operation part/get/complete及MCP会话；拒绝旧上传、版本枚举、引用、下载链接、文件写/删除/恢复、标签写/人工决定/归档、operation列表/resume/cancel、管理/分享与错误方法路径。撤销后所有原允许入口拒绝，认证保留tenant/space/Agent上下文。与缺失/未知profile和stale Principal回归通过temp/integration-test-runs/20261002T061154Z-db825a9a/；测试无LSP诊断。此为认证函数矩阵，不冒充每个真实HTTP handler/worker执行链；后端基础仍需应用层入口审计收敛。

## B02.2 SQLite迁移与降级流程验收登记（2026-10-02）

当前服务层、SDK、server命令普通全量通过temp/integration-test-runs/20261002T060635Z-b385257d/，pure-Go构建通过temp/integration-test-runs/20261002T060933Z-cb047779/。结合独立测试进程迁移恢复、旧列权限保护及停写后降级预检，经任务CLI登记B02.2完成。范围是SQLite增量迁移和受控运维降级流程；不是旧binary兼容认证，不能阻止绕流程启动，强杀/掉电、PostgreSQL/多实例/规模未验收。下方“B02.2未完成”为当时历史记录。本目标backend-foundation仍需完成全入口restriction保护核对，不因两模型任务完成提前标记；完整HTTP进程恢复归后续集成任务。

## 降级运维预检（2026-10-02）

新增metadata check-workspace-downgrade复用服务查询：存在任何未撤销grant（含disabled/expired）拒绝；无记录/全撤销通过；DB故障失败关闭。服务和真实Cobra命令测试确认不自动撤销或改revision。证据temp/integration-test-runs/20261002T060455Z-9f3c1b91/，四文件无LSP错误。runbook明确先停签发/worker、命令正常打开服务可能迁移、成功不认证其它schema、不允许部署且不能拦截任意旧binary；未执行真实库检查。B02.2尚未登记完成。

## 独立进程迁移恢复（2026-10-02）

新增TestWorkspaceMigrationRecoveryAcrossProcesses：三个独立测试二进制进程共享临时SQLite文件，依次真实唯一约束迁移失败、确认回滚与旧身份/冲突行保留并显式修复fixture、再次启动重复迁移与回读。证据temp/integration-test-runs/20261002T055939Z-7217b025/；新测试文件无LSP诊断。仅证明迁移数据库跨正常进程退出恢复，不是强杀/掉电、旧binary降级或完整HTTP服务重启验收。B02.2保持未完成。

## 当前可验证结果

- B01.1–B01.3通过项目任务CLI登记。Go DTO生成OpenAPI、TS与fixture；make contracts/check；Go/Web/SDK/隔离MCP transport消费同源样例。许多schema路由仍planned，不把这些当已实现功能。
- GORM增量模型：WorkspaceGrant、AgentGroup/Organization、Submission/Decision、Request、CredentialHistory、LabelSuggestion/SuggestionDecision。启动迁移已接入；SQLite重复/部分迁移、旧Agent列升级、唯一约束、回滚与数据库重开测试存在。
- Agent增加不出现在旧JSON的RestrictionProfile；新身份无旧scope。未知profile或grant丢失不得退回旧权限。
- 人类HTTP已接：POST agent-grants及lookup、GET单grant、PATCH单独enabled或expiry、revoke、rotate及rotate/lookup。带认证/CSRF/作用域；变更同事务写CAS/回执/审计，secret一次返回；注入审计失败可回滚。仅name的PATCH现已接原事务框架：修改Agent显示名并CAS递增grant revision，不改期限/token/scope/credential_generation；审计失败回滚Agent/grant/receipt，原键重放不再次更新。定向证据temp/integration-test-runs/20261001T175059Z-812a6672/，三个检查文件无LSP诊断。prefix/actions内部patchWorkspaceGrantScopeOn已实现：当前人类/空间/新旧prefix权限复检，grant与Agent目录绑定同事务CAS更新，动作收缩立即作用于专用授权，不改token/期限/代次；事务回滚、旧revision、目录穿越与旧scope拒绝测试通过temp/integration-test-runs/20261001T175439Z-7fe3f5d7/，两个文件无LSP诊断。prefix/actions HTTP已接原键摘要回执及事务审计；同键重放不恢复后来收缩的目录，审计失败回滚Agent/grant/receipt。真实旧凭据先可submit，收缩为suggest后原凭据submit立即拒绝且token hash不变；证据temp/integration-test-runs/20261001T180334Z-388238aa/。此前175913Z-a82c31fa失败为测试旧Agent补marker但未补连接绑定，已修fixture，不放宽生产授权。组合PATCH现已接统一字段事务：名称/目录/动作/enabled/expiry一次CAS只递增一次revision，期限仅显式提交时更新，单字段既有回执域保留，组合使用grant.patch。HTTP测试覆盖五类字段同时提交、审计故障全量回滚、同键重放不续期、旧revision/变更payload拒绝、token不变与单回执；证据temp/integration-test-runs/20261001T181407Z-bf9e12c1/，五个检查文件无LSP诊断。另补duration→never+enable组合，验证SQL/响应清空expires_at、保留issued_at、重放不改变expiry_changed_at；过期grant启用+改never+改名403且无部分写入。Grant/Rotation/DedicatedAction命名用例集中通过temp/integration-test-runs/20261001T181707Z-396b422f/，变更测试无LSP诊断。尚未据此验收完整grant生命周期或最终全量。
- 新Agent认证开放专用标签建议POST、收件创建/lookup/单条读取、原operation的part/complete/单条恢复；其它入口保持拒绝。内部专用授权逐次复读owner/space/Agent/grant、profile、token hash、期限与action。
- POST label-suggestions/{id}/decision已接人类认证，原建议子集增量合并manual标签，不覆盖已有人工行；固定版本/CAS/幂等/审计在事务内。

- 人类收件决定POST已接通：固定version与revision CAS，accept/reject不移动或删除文件；原键重放、同键改决定409、Agent自批拒绝、审计失败回滚决定/回执/状态。读取返回持久化决定。局部HTTP与内部测试证据：temp/integration-test-runs/20261001T150343Z-283b2a41/；五个检查文件无LSP诊断。
- 所有TestWorkspace命名用例集中回归通过（不是server全量）：temp/integration-test-runs/20261001T150446Z-1d7d1785/。未据此提前完成B02或S2，归档/MCP/前端等仍待交付。

## MCP 接线当前边界

HTTP安全operation回读已提取workspaceOperationRecovery共享应用方法。CallTool对带restriction marker的Agent默认拒绝，仅允许operation_get和complete_upload走现有专用授权与安全进度投影；撤权后回查拒绝，完成重放保持原操作且不输出path/file/version/connection。定向证据temp/integration-test-runs/20261001T161216Z-aa31bb25/。最初测试缺ServiceIdentityBinding迁移导致database_unavailable，补齐fixture后通过（失败证据160840Z-42962477）。后续已开放/mcp会话认证，但CallTool仍按受限白名单裁决。补齐此前只有CallTool分支/资源、未注册工具的capabilities定义；新profile返回当前授权HTTP创建/lookup/分片恢复合同与已实现恢复工具，files_read_ready=false。真实Streamable HTTP客户端已验证初始化、capabilities发现、旧create_upload拒绝、operation_get、complete_upload及撤权后会话拒绝，复用同一HTTP收件/人工接受/归档链；旧MCP身份测试同时通过。证据temp/integration-test-runs/20261001T162023Z-06bc437a/，六个检查文件无LSP错误。失败161745Z-76fe1a69为capabilities未注册，已补齐。tools/list现使用SDK原生WithToolFilter按当前受限Principal投影：submit仅capabilities/operation_get/complete_upload，收缩为read-only仅capabilities，撤权无工具；旧身份保留既有列表。真实MCP列表和旧MCP回归通过；权限收缩与撤权的应用层发现/执行检查通过，证据temp/integration-test-runs/20261001T162550Z-325efdd8/。双Agent收件隔离已新增真实HTTP+MCP测试：同owner/前缀/原键分配不同submission/operation，lookup按grant隔离，双方HTTP及MCP跨操作回查/complete拒绝，另一用户404，撤销A不影响B完成，owner可回读两个submitted结果。使用零字节文件覆盖无part完成分支；证据temp/integration-test-runs/20261001T162811Z-51bf388f/，新增测试无LSP诊断。随后同一测试扩展empty/with_part子例：双向跨Agent PUT返回403且不留Part，本Agent分别落库两个Part，无part时MCP complete拒绝，随后各自完成；证据temp/integration-test-runs/20261001T163021Z-c9a625fe/，测试文件无LSP诊断。服务关闭/重开与HTTP、MCP重连已验证：分片后关闭旧Handler和Service，用同Config/磁盘state重开，保留OIDC/S3 fixture依赖；同键lookup保持原submission/operation、received_parts保留，继续MCP complete、人工接受和故障归档恢复。证据temp/integration-test-runs/20261001T163820Z-1eb6e600/。此前163633Z-c579fd17失败是重开后测试登录依赖fixture表单，已将两个用户预先登录，未给生产Handler添加测试入口。测试无LSP诊断。这不是独立OS进程重启，仍需进程级验收。

## 分组内部事务进度

新增deleteWorkspaceGroupOn：人类owner/tenant隔离、group revision校验，删除分组与解绑本域组织记录同事务，保留pinned并递增组织revision，不改变Agent/token/scope；回滚保留原绑定。测试同时验证Agent/跨租户/旧revision拒绝和不触碰外域元数据。证据temp/integration-test-runs/20261001T164500Z-408edfe9/，两个新文件无LSP诊断。创建/改名内部服务现已实现：trim后1–80 Unicode字符、拒绝控制/无效UTF-8，lowercase名称在owner+tenant域唯一；改名revision CAS、冲突/跨owner/Agent拒绝、回滚不消耗revision。创建/改名/删除共用人类身份检查，定向全部TestWorkspaceGroup通过：temp/integration-test-runs/20261001T164933Z-80423e4e/，三个检查文件无LSP诊断。GET /agent-groups/{id}已接真实HTTP，仅人类owner/tenant可读，Agent、跨用户/租户与停用owner拒绝；真实fixture测试与已有Group定向用例通过，证据temp/integration-test-runs/20261001T165250Z-3e23d532/，两个新增文件无LSP诊断。POST /agent-groups现已接入身份/CSRF、原键摘要回执及事务审计，201返回GroupView；同键回原group、同键改payload与规范化同名409，审计故障回滚group/receipt，重放不重复审计。全部Group定向测试通过temp/integration-test-runs/20261001T165611Z-61f39fed/，两个新文件无LSP诊断。DELETE /agent-groups/{id}现已接人类身份、revision、删除/解绑/回执/审计同事务；组不存在后原键仍可返回删除结果，改请求409、外owner404、停用后重放403。HTTP注入审计失败验证解绑回滚，同键成功重放不再次递增组织revision且保留pinned。全部Group定向通过temp/integration-test-runs/20261001T170007Z-86c5c0e3/，两个新文件无LSP诊断。PATCH改名HTTP现已接通：人类owner/tenant检查、revision CAS、回执与审计同事务；同键重放不递增revision，同键改名/旧revision409、停用后重放403，审计故障回滚原名/revision。全部Group定向通过temp/integration-test-runs/20261001T170425Z-c11598e5/，两个新增文件无LSP诊断。GET分组列表已接live keyset分页，复用加密游标并绑定owner/tenant/context/limit；空分组可见，不返回成员计数，total=null/unknown。真实HTTP验证两页身份、跨owner游标/变更limit拒绝、畸形参数拒绝、Agent与停用后分页拒绝。全部Group定向通过temp/integration-test-runs/20261001T170837Z-1e462967/，两个新增文件无LSP诊断。归组/置顶内部patchWorkspaceOrganizationOn已实现：支持旧Agent身份，首次expected_revision=0、后续CAS；group_id=null解绑、省略保留，pinned=false显式生效；校验人类owner/tenant/space及目标group，绑定时锁定group。串行归组→解绑→重归组→删组保留置顶、Agent权限不变，以及外租户/空间/Agent拒绝测试通过：temp/integration-test-runs/20261001T171329Z-aad52f44/。两个文件无LSP诊断。组织PATCH HTTP已接当前人类/Agent空间权限复查、原键摘要回执及事务审计；审计失败回滚、旧请求重放不覆盖更新后的解绑，停用后重放403。已删除本人group由同owner/tenant删除回执判409，未知/外域记录404。删除与归组统一锁group；SQLite单连接同时归组/删除验证最终无悬空绑定且保留pin（不声称多实例/PostgreSQL并发）。GET单条组织已接，缺行返回虚拟revision0/null/false且不写库，刷新回读持久状态。证据temp/integration-test-runs/20261001T172736Z-a01e38e1/，读取与HTTP测试无LSP诊断；此前PATCH证据172057Z-197b5f8e，并发证据172346Z-7c339619。组织分页GET已接：包含旧Agent虚拟revision0行，owner/tenant/space过滤后按pinned降序、agent_id升序，支持group/ungrouped/pinned；加密游标绑定筛选/context/limit及可见结果摘要，排序变化拒绝旧游标。测试覆盖两页、虚拟行筛选、不写虚拟行、跨owner游标和非法筛选，证据temp/integration-test-runs/20261001T173214Z-5bb77da5/。q.Find的无WHERE告警已按实际owner/tenant预过滤记误报。当前读取本人可见集合后排序分页，规模性能未验收；新分组/组织列表失效游标现统一返回cursor_expired/410，不改变旧组织查询；跨owner、筛选/limit或排序变化的HTTP测试通过temp/integration-test-runs/20261001T173806Z-f21d98ab/，四个检查文件无LSP错误。B15完整验收未完成。

## Operation 旧表升级证据

新增legacy Operation schema fixture，先写旧表原操作再两轮迁移；验证purpose/grant默认空，不重分类旧操作，owner/Agent/key/fingerprint/固定版本/连接/路径/进度/时间不变，原owner+key唯一性保留且不同owner同键可用。与旧Agent列升级一起通过：temp/integration-test-runs/20261001T164116Z-f972d00d/。新增测试无LSP诊断；不将此数据库升级用例当作进程重启。

## 标签建议固定版本回读

GET /files/{id}/label-suggestions已接：version_id必填，当前权限复检，人类读该可见固定版本所有来源，新Agent仅自身建议；返回creation_key用于丢响应恢复，不附送正文读取权限。游标绑定owner/tenant/context/Agent/file/version/limit及可见建议代次，失效410。真实双Agent流程补回读、外owner404、缺版本拒绝与撤权后拒绝，通过temp/integration-test-runs/20261001T182247Z-ef79fb19/。q.Find无WHERE为静态误报（已有owner/tenant/file/version/Agent条件），已标记；规模性能未验收。

## 收件列表最新结果

GET /inbox-submissions已实现state/space/agent筛选与游标分页；复用单条读取授权，人类owner/tenant/空间过滤，Agent仅自身来源，逐条权限过滤后分页。HTTP+MCP双Agent空文件/带part场景验证列表隔离，人类两页无重复，Agent复用人类游标拒绝、complete改变状态后旧游标410；服务重开链回归通过。最新证据temp/integration-test-runs/20261001T183440Z-ed4cbfe6/。标签建议分页也已验证人类两来源翻页、Agent拒绝人类游标和采纳后410，证据temp/integration-test-runs/20261001T182543Z-830d6395/。当前列表按本人集合投影、逐条授权再分页，存在规模与查询次数优化欠账，不声称性能验收。未完成B02或整体目标。

## Grant 本地检查

POST /agent-grants/{id}/checks已接：人类owner/tenant/permission与expected_revision校验；仅报告grant_active、identity_binding、protocol_configured、storage_configuration本地事实。ConnectionState始终unknown，不探测远端、不创建Operation、不更改期限/revision/代次。HTTP测试覆盖正常/停用事实、旧revision409、跨owner404、Agent拒绝和状态不变，证据temp/integration-test-runs/20261001T180732Z-b2e16944/，两个文件无LSP诊断。

## files.read 内部固定版本授权

新增workspaceReadVersionOn及workspaceVersionReader：必须固定version，每个最大256KiB读取块前复读当前文件路径、版本归属、grant和token，治理未开启也检查撤权；不开放旧scope/staging。历史固定版本可授权、打开流后撤权返回零字节错误、文件移出prefix或进入staging拒绝测试通过temp/integration-test-runs/20261001T191930Z-76053788/，两个文件无LSP诊断。现已复用downloadResolvedVersion的存储/解密实现，专用downloadWorkspaceVersion先事务授权、流读取持续复查；GET /files/{id}/content仅新profile显式单一version参数开放。真实HTTP+SDK字节回读、缺version/仅suggest/staging拒绝、旧人类及加密下载回归通过192654Z-d16c9964。发现新增fixed_version_content_ready及固定version GET合同，但files_read_ready仍false（列表/搜索未接）。下载空间查询失败不再默认按未加密内容输出，关闭body并返回database_unavailable；故障注入与下载回归通过temp/integration-test-runs/20261001T193025Z-e9ba209b/，四个检查文件无LSP错误。新增单文件GET /files/{id}走同一固定版本专用授权并返回旧形状file/ref；MCP stat已注册并仅向files.read展示，现支持file_id+可选version_id或固定ref，混合selector及跨实例ref拒绝；固定版使用该Version的size/MIME/created，未固定的HTTP stat保留当前File元数据。真实MCP固定历史引用不替换为当前版及metadata回归通过temp/integration-test-runs/20261001T195538Z-aee5a8cf/。真实Streamable HTTP验证metadata含固定version，移入staging与动作收缩后同会话stat拒绝，旧MCP/收件链回归通过temp/integration-test-runs/20261001T194647Z-ccd0112d/；六个检查文件无LSP错误。HTTP GET /files现已对新profile走searchWorkspaceFiles，保留旧数组形状；先授权grant，再按当前版本连接/目录逐项校验，支持q/prefix/mime/after，拒绝trash和越界prefix，排除staging。内部查询及真实HTTP列表/搜索测试通过temp/integration-test-runs/20261001T200248Z-42059f54/，三个检查文件无LSP错误。MCP files/search现已接同一searchWorkspaceFiles，并按files.read展示capabilities/stat/files/search；能力发现补HTTP文件列表合同，files_read_ready按当前专用授权投影。真实Streamable HTTP验证files/search返回授权文件、越界path拒绝、staging无结果，旧MCP回归通过temp/integration-test-runs/20261001T200841Z-0b21ddf7/，四个检查文件无LSP错误。查询仍全量候选+逐条授权，规模未验收。完整files.read和B05仍需验收，不由ready字段推断所有入口已验证。

## 轮转修复旧列宽权限残留

确认并修复缺profile标记绑定的轮转路径：此前只补profile/hash，可能留下旧scope与未来expires。轮转现在在同一CAS事务清空scope并将未来旧期限收紧至now，不延长已有旧期限；WorkspaceGrant期限保持不变。扩展原轮转测试先失败（temp/integration-test-runs/20261001T203502Z-db26fcef/），修复后轮转/签发/旧列保护回归通过temp/integration-test-runs/20261001T203534Z-7f987dc3/。失败事务保留原hash/profile/scope/expires，成功事务关闭旧鉴权。两文件LSP无未抑制诊断，工具报告一条既有被抑制发现，不能声称零诊断。不是旧binary降级执行验收。

## B02 旧鉴权投影保护

新增旧Agent列投影回归：新grant签发、凭据轮转、改为365天期限后，旧列始终scope为空且expires不在未来；新grant期限/代次保持独立有效。证据temp/integration-test-runs/20261001T202545Z-161b390f/，测试文件无LSP诊断。这是旧列鉴权不变量测试，不是旧binary执行或完整降级审批/阻断验收；B02.2不能据此直接完成。

## B02 收藏记录模型

新增服务端WorkspaceFavorite最小GORM记录，owner+tenant+space+kind+完整target_ref唯一，kind仅file/directory/tag/collection；无目标正文/授权字段，不存桌面ReadingState。fixture使用显式带connection/root的目录引用验证同路径跨连接/root不合并，重复绑定拒绝、跨owner/tenant/space隔离及重复迁移保留身份。证据temp/integration-test-runs/20261001T202147Z-980baf13/，三个文件无LSP诊断。该模型不验证引用语法；后续应用层必须复用统一canonicalization/CAS规则，不能把fixture引用示例当已发布wire合同。尚无Favorite API或桌面Pinax写入；B09/D04不因此完成。

## B02 资源描述与只读检查模型

新增WorkspaceResourceDescriptor，CHECK约束只允许file+version或snapshot二选一，kind仅skill/template/reference_pack；新增WorkspaceResourceCheck，descriptor+revision+checker_version唯一，事实状态不包含安装/执行。固定引用、无效组合、重复/历史检查与重复迁移测试通过temp/integration-test-runs/20261001T191405Z-de82d13c/，三个检查文件无LSP诊断。尚无资源解析/检查执行/API，不声明资源功能完成；没有外部访问或安装行为。

## B02 固定版本链接模型

新增WorkspaceFileLinkProjection，源file/version+target_ref+anchor唯一，owner/tenant/space索引用于后续读取；不缓存目标标题/正文，不赋予目标权限，不读Pinax关系库。重复、跨版本/文件/anchor身份及重复迁移测试通过temp/integration-test-runs/20261001T185839Z-208ae0d3/，三个检查文件无LSP诊断。尚无解析器或HTTP链接查询，不能声称知识链接功能完成。

## B02 属性模型与迁移失败恢复

新增WorkspacePropertyDefinition（space+stable key唯一、类型约束）与WorkspaceFilePropertyProjection（file+version+definition revision+key唯一、storage_kind约束），增量注册迁移；不接属性写入API，不涉及Pinax本机状态。未知JSON值与固定版本跨重复迁移保留，证据temp/integration-test-runs/20261001T185033Z-0af1f922/。

migrateWorkspaceModels已包事务。旧fixture属性表故意含重复stable key，真实AutoMigrate晚期失败：此前新建grant/request表回滚，旧Agent身份及冲突行原样保留；仅测试显式删除重复fixture后，重试及重复迁移成功。与属性迁移和服务重开链一起通过temp/integration-test-runs/20261001T185426Z-be93e0ee/，两个检查文件无LSP诊断。仅SQLite证据，非PostgreSQL或真实数据迁移；B02还需其它metadata模型/降级保护核对。

## 新profile文件列表参数校验

HTTP /files仅新profile严格拒绝重复参数、未知参数及非true/false的trash值，避免静默忽略筛选；允许既有tenant_context/space_context，旧身份列表行为不改。重复space/prefix、未知筛选及非法trash负例和读取/旧MCP回归通过temp/integration-test-runs/20261001T202914Z-a009f6ca/，两个检查文件无LSP错误。

## 固定版本读取故障传播

workspaceReadVersionOn已区分File/Version真实缺失与数据库错误，后者返回database_unavailable，避免列表把基础设施故障当作不可见行。两查询点故障注入与缺失对照、流授权及HTTP/MCP读取回归通过temp/integration-test-runs/20261001T201226Z-f947da1b/，两个检查文件无LSP诊断。仍不声称其它授权查询都已完成错误分类。

## 列表授权错误传播

Grant与Organization列表不再无条件吞掉governanceAllow错误；仅明确不可见拒绝可跳过行，database_unavailable/unauthorized/未知错误向上传播。分类单元测试与两列表HTTP回归通过temp/integration-test-runs/20261001T190257Z-d89fe83c/，四个检查文件无LSP错误。后续修正governanceAllow的Space/Team查询：仅ErrRecordNotFound映射not_found，其它错误映射database_unavailable。Space查询故障注入与真实缺失对照测试、既有治理流程回归通过temp/integration-test-runs/20261001T190631Z-4a49ba58/，两个检查文件无LSP错误。Grant列表处理器故障注入已补：候选空间枚举成功、逐条授权Space查询失败，处理器返回database_unavailable且尚未输出成功body；既有writeError映射500保持不变。证据temp/integration-test-runs/20261001T191012Z-26effd0f/，新增测试无LSP诊断；190903Z-c556b329失败为测试误期望503，已按既有合同修正断言。此为处理器+错误writer测试，不冒称真实HTTP网络全链故障；scopeRole等其它底层错误映射仍需核对，不能声称所有基础设施失败均已正确传播。

## 最新权限收敛与回归

- MCP capabilities已补收件列表与固定版本建议列表发现参数，保持HTTP字节传输与自身来源限制；证据temp/integration-test-runs/20261001T183807Z-ce0e8cfe/。
- CallTool与tools/list不再信任缓存Principal中的空restriction标记：先检查持久化grant及Agent标记，关联损坏或旧快照不得回退旧工具；真实旧MCP与新收件链回归通过temp/integration-test-runs/20261001T184411Z-82bdaaa1/。
- 当前全部TestWorkspace命名用例通过temp/integration-test-runs/20261001T184521Z-13686d38/。此结果不是当前server全量；后端基础仍需逐项验收，不能由局部覆盖推断所有HTTP/worker入口安全。

## 最新列表与集中回归

- 分组游标已由固定live改为本人group id/revision摘要；创建/改名/删除后410，其他owner改动不影响本域游标。证据temp/integration-test-runs/20261001T174138Z-c62f78fe/。
- GET /agent-grants已接：本人/租户/空间授权过滤，state和名称q筛选，加密游标绑定查询及可见GrantView代次；变更后410、Agent拒绝。复用批量Agent读取，不输出credential。定向证据temp/integration-test-runs/20261001T174453Z-4d2e0b69/。当前与组织列表一样先投影本人集合再分页，规模性能未验收。
- 当前全部TestWorkspace命名用例集中通过：temp/integration-test-runs/20261001T174601Z-3c6bde1d/；这是workspace定向回归，不是当前server全量，不推进未满足的B02/B15/目标里程碑。

## B02.1 本端验收登记

已按实际任务范围核对grant/submission/group/organization及metadata增量唯一约束、never/duration可空期限、租户用户隔离、旧Agent/Operation身份保护；当前服务层+SDK普通全量通过temp/integration-test-runs/20261001T203736Z-317ef9b0/（含Favorite与最新轮转修复）。通过scripts/openspec-tasks.py将contracts-v1的B02.1登记完成。仅BACKEND模型任务完成；B02.2降级/重启独立未完成，backend-foundation目标里程碑仍进行中，前端INTEGRATION_PENDING，总目标仍2/11。

## 当前稳定后端边界（历史全量）

2026-10-01当前服务层与SDK普通全量测试通过：temp/integration-test-runs/20261001T201440Z-f4864ff5/（go -C server test ./internal/service ./sdk -count=1 -timeout=5m）。当前pure-Go服务端构建通过temp/integration-test-runs/20261001T201756Z-1c9dd5ba/。未运行race。这覆盖本轮之前全部后端改动，但不等于所有S0–S4功能已实现；B02仍缺Favorite归属/模型及降级保护核对，前端壳/主要业务UI尚未交付。不得把当前全绿当目标完成。

## 最近集中验证与待接线

- 服务层和SDK普通全量通过：`go -C server test ./internal/service ./sdk -count=1 -timeout=5m`；证据 `temp/integration-test-runs/20261001T153128Z-3eae3550/`。没有运行race。
- 当前pure-Go服务端构建通过：`temp/integration-test-runs/20261001T153434Z-61c41813/`。
- 首次全量被外层120秒中断，`151443Z-b83f9749`证据不完整，不算通过；随后`151725Z-65c6e4fa`记录四项失败。UTC对照通过，定位混合时区时间文本比较问题；改为读取授权记录后比较Go时间瞬间，未放宽过期权限。涉及governance_model.go、governance_recovery.go、model.go；authorization_time_test.go覆盖UTC±8及UTC的有效/过期session、access、range。最终全量已包含修复。
- 归档已完成内部准备校验和同名冲突策略：`archiveVersion(..., autoRename=false)`复用原服务，旧ArchiveVersion仍自动重命名；准备函数仅允许本人类对accepted固定版本归档，拒绝跨用户/租户、staging目标和旧revision。原归档operation创建已通过内部workspaceArchiveIntent与submission原子绑定：accepted→archiving revision4、审计和配额预留同事务；发布时finishWorkspaceArchiveOn将archived/file引用与目标File/Version同事务提交。定向测试注入两阶段审计故障，验证无半成品、原操作重试与审计不重复，证据temp/integration-test-runs/20261001T154256Z-01b7a44b/。四个相关文件LSP无诊断，files.go仅原位置四项N+1告警。此修改发生在上述全量之后，目前只完成定向验证。收件归档HTTP现已接通：原键/摘要/operation回执与创建事务一起持久化；重放先复查人类/租户/源与目标权限，返回原操作、不重复调度传输。202返回operation_ref和必返可空file_ref。真实本地HTTP+SDK+SQLite链已覆盖创建→分片→complete→人工接受→归档→重放与archived回读，Agent自归档及同键更改目标拒绝；证据temp/integration-test-runs/20261001T155059Z-0658983f/。五个检查文件无LSP诊断。归档存储Put后Get回验故障的恢复现有定向证据：原操作保持verifying且submission仍archiving，回执找回原ID；Resume对verifying的archive/copy直接Complete而不重发被拒绝的分片；原operation恢复后提交archived目标引用、单一版本。归档发布事务新增当前人类Disabled/Archived复查，禁用后恢复不发布目标，重新启用再恢复成功。证据temp/integration-test-runs/20261001T155954Z-aae34b5c/，三个改动文件无LSP诊断。该故障注入使用存储测试适配器，HTTP归档发布审计故障恢复已补：首请求503但原operation/submission绑定保留；同键202返回原operation且file_ref=null，随后原/operations/{id}/resume完成，重放与archived回读返回同目标。另一用户resume 404、submit Agent恢复人类归档拒绝，只有一operation与一目标version。证据temp/integration-test-runs/20261001T160349Z-717a928e/，测试文件无LSP诊断。这覆盖发布事务故障，不代表所有网络/进程中断；跨实例并发、MCP和进程重启仍待验收，不把这条单用户fixture链当完整S2。
- 专用授权已拒绝staging上的files.read与labels.suggest，即使grant包含三个动作；inbox.submit仍需operation/submission绑定校验。定向证据 `temp/integration-test-runs/20261001T151436Z-8f63ca9d/`。

## 最新闭环证据

`server/internal/service/workspace_suggestion_flow_test.go`：本地OIDC登录→HTTP签发两个suggest-only grant→SDK真实HTTP提交同键的独立建议→人类采纳→撤销第一Agent→第一拒绝且第二仍能重放。目录外、旧上传、旧标签写、正文读取、自批均拒绝。注入提交审计失败不留proposal。文件与版本为fixture预置，不冒充上传/收件完成。

最后通过：`temp/integration-test-runs/20261001T140213Z-4222ec95/`（suggestion flow及decision HTTP，CGO_ENABLED=0）。此前核心证据引用见backend-foundation-progress.md；不必逐条重读历史。

## 紧接着的工作

1. 跨用户文件与外来version绑定负例已加入建议闭环并通过：temp/integration-test-runs/20261001T140446Z-1c9fc747/。错误身份上下文负例仍待补，勿重复已完成检查。
2. 收敛B02基础验收，按实际任务检查尚缺的持久模型/迁移保护，而不是无限扩展零散测试。完成前确认普通pure-Go构建；不要重复全项目测试。
3. 原Operation已增量加入Purpose/WorkspaceGrantID，默认空保持旧操作；旧operation读取/写/transfer及publishUpload拒绝所有非空绑定（含残缺或未知purpose），避免以后staging借旧权限发布。重复迁移保留绑定，旧上传恢复用例通过，证据temp/integration-test-runs/20261001T141747Z-318d208b/。内部createWorkspaceSubmissionOn现已在同一事务分配submission+原Operation+配额预留，生成grant前缀下隔离路径，不复用blob或发布File；原键同摘要复用原身份，撤权重放拒绝，失败回滚不留操作或预留。workspaceOperationOn复核operation/submission/grant/Agent、tenant、prefix、connection与当前凭据；残缺绑定/外来操作拒绝。定向证据temp/integration-test-runs/20261001T142224Z-355dae3b/，四个新增文件LSP无诊断。POST /inbox-submissions与/lookup已接HTTP精确认证入口及事务审计，创建201返回原操作传输参数；局部互斥与Space行锁保护配额分配。HTTP+SDK+SQLite验证真实签发submit-only→创建/重放/lookup、不同摘要冲突、注入审计失败回滚配额、旧uploads拒绝与撤权lookup拒绝；证据temp/integration-test-runs/20261001T142630Z-26cc3f13/。PUT原operations/{id}/parts/{index}已按新profile路由至专用绑定授权，复用原分片字节/加密实现；接收字节后及Part事务内复查grant。内部测试覆盖读流途中撤权且无Part落库，HTTP验证分片持久化/重放/冲突/撤权拒绝，旧恢复用例通过：temp/integration-test-runs/20261001T143229Z-72ee3457/。事务失败移除未提交分片文件。GET原operations/{id}已为当前grant提供安全进度及received_parts恢复，旧operations列表仍拒绝；专用GET及分片响应不返回path/connection/grant/file/version/正文，no-store。HTTP测试验证空/已持久分片回读和撤权后拒绝；证据temp/integration-test-runs/20261001T143655Z-14c7229f/，三个窄文件LSP无诊断。GET /inbox-submissions/{id}已支持自身grant和本人类owner回读，跨用户404、撤权Agent拒绝但人类仍可恢复；分片完毕仍诚实返回uploading且无FileRef，证据temp/integration-test-runs/20261001T144043Z-2c8e3635/，三个窄文件无LSP诊断。finishWorkspaceSubmissionOn内部发布尾事务已实现：重新授权、核对原op与File/Version一致、CAS uploading→submitted revision2、审计同事务；人工构造的元数据发布测试覆盖审计失败/撤权回滚File/Version/submission、原固定引用与幂等不重复审计，证据temp/integration-test-runs/20261001T144517Z-6a7a9adb/。内部completeAuthorized已复用真实拼装/摘要校验/存储Put/Get回验/发布事务，专用分支不借旧write；publish事务重新检查原op/submission绑定并调用finishWorkspaceSubmissionOn。既有memoryStore fixture+SQLite验证完整字节、固定版本、配额只结算一次，注入提交审计故障时File/Version/Blob/submission/配额回滚，原operation可重试；证据temp/integration-test-runs/20261001T144942Z-ab6332c7/。该存储为测试适配器，不冒称真实云或HTTP complete完成；HTTP开放前仍需staging旧读入口隔离审计。HTTP complete现已接入专用授权路径，本地HTTP+SDK+SQLite从真实grant创建→分片→received_parts→complete两次→submitted/file引用回读通过；配额只结算一次且只一版本。旧Agent的staging文件/content被allowOn拒绝，orgAgentScope在分页/计数前排除staging；对应定向测试通过。证据temp/integration-test-runs/20261001T145718Z-d4adadcf/，此前145622Z-28dd8194失败为新增测试漏填Mode=exact，已修复。存储仍为本地测试fixture，不是云；MCP、归档和进程重启尚未完成（人类accept/reject现已接通，见上方最新证据），不称完整收件闭环；跨实例并发配额和创建幂等竞争未验证。新/改三个窄文件LSP无诊断，http.go四项未触碰位置告警保留。继续实现受限inbox.submit链：服务端选择staging→原operation→分片/complete发布前授权复检→submission固定版本；不能开放旧create_upload或宽write。必须先读既有CreateUpload/PutPart/Complete与运行时发布流程。
4. 新Agent files.read与MCP发现/恢复需独立接线和越权测试；不能仅放开authenticate就认为所有旧端点安全。
5. 前端新壳、完整Agent组织、S1/S3/S4尚未交付。不要把目前后端局部功能报告为项目完成。

## 风险与核对

- 当前SQLite单连接并发证据不证明PostgreSQL并发。普通上传publishUpload已增加事务内write/overwrite授权复检；定向钩子在外层授权后、事务前撤权，验证拒绝且不写File/Version。与既有blob instant/concurrent complete测试一同通过：temp/integration-test-runs/20261001T141326Z-08832a6d/。这只覆盖该撤权窗口，尚不是新收件purpose、grant修订或跨实例锁的完整发布fencing。files.go静态检查仍报告未改动978–1010行的四项N+1告警；新增测试无诊断。
- 生成OpenAPI整体仍planned，与已有部分handler的就绪标记尚需最终一致化。
- 新权限中space.ConnectionID严格匹配，更多连接场景需要依据既有合同处理，不能放宽为任意连接。
- 人工标签采纳与既有SetFileLabels并发一致性需后续集成核对；当前采纳测试并不覆盖全部旧写入口竞争。
- 大量既有dirty/untracked迁仓文件及桌面并发设计必须保留；不要使用git reset/clean或依赖HEAD覆盖文件。
- 最近文件工具曾返回超出limit的内容；尽量按符号或精确小范围读取，不反复读取长进度日志。

## 定向命令

从cli/drivebridge运行：

```bash
CGO_ENABLED=0 python3 scripts/check.py -- go -C server test ./internal/service -run 'TestWorkspaceSuggestionAuthenticatedFlow|TestWorkspaceSuggestionDecisionHTTP' -count=1
make contracts-check
```

业务实现仍按当前AGENTS.md和已确认目标推进；本页不授予真实凭据、外部调用、发布或S5权限。
