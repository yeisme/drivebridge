# 前端重建保护基线

本记录对应 F01.1 的重建前保护清单。工作树、公开入口、截图、恢复与权限反例基线已核对；这里只验收基线保护，不声称新壳或全部业务完成。

## 空目录范围撤销保护

useDirectoryRead在space清空时清entries/next_cursor/total并结束loading，不发空范围请求；旧adapter忽略abort后的响应不能回填。三个hook测试192340Z-abb70fc1及tsc192343Z-75d427d7通过。本片尚未重建浏览器验证，不能把空范围保护算作新版FileBrowser完成。

## Finder目录读取职责迁移

新增features/files/useDirectoryRead承载BrowserEntry/DirectoryPage及目录查询effect；旧Finder调用该hook，保留scope/page/sort/cursor、200ms查询延迟、本机adapter和取消后结果/错误屏障，reader变更也重启读取。旧BrowserEntry导出保留。tsc191730Z-64e9be29、四个hook/FilesPage用例191907Z-ac9049c4、构建191910Z-935bb94b通过；独立端口Finder目录/文件不覆盖、原预览兼容、本机CLI view三浏览器例通过 `temp/integration-test-runs/20261002T191957Z-dde2ec7e/`。这是实际读取职责迁移，旧Finder UI/操作仍在，不冒称新版FileBrowser完成。

## 网盘连接观察状态保真

DriveHome不再将缺失连接显示为已配置或在线绿点，统一connectionObservation：缺失未知、disabled已断开、verified已验证、其它仅已配置尚未验证；仍允许进入原受权网盘，不用前端状态代替服务端权限。4个DriveHome/Workspace用例191039Z-17f2a059、tsc191044Z-0427b4dd、构建191131Z-70f11fbd通过；Finder和真实本机view两浏览器例通过 `temp/integration-test-runs/20261002T191222Z-e1feeb55/`。该显示为已有观察，不主动连接云端。

## 管理与分享对话框脱离Finder转导出

Sharing/UserDirectory/LocalSharing/Teams/KeyDirectory/CreateIdentity/Governance直接导入原AppDialog并沿用WorkspaceDialog局部名称，不再为弹层加载Finder；旧Finder转导出保留兼容。tsc190350Z-7d13fef5、10个定向测试190407Z-63be31d9、构建190432Z-6f7ed697通过；独立team fixture五浏览器例190520Z-522390ea，以及Key/本机view/Finder兼容四例 `temp/integration-test-runs/20261002T190629Z-2d73843f/` 通过。不改变对话框原语或发布真实分享，仍为旧Finder替换前的依赖收敛。

## 文件入口与旧Workspace依赖解耦

DriveHome/filterSpaces迁到features/files/DriveHome，FilesPage与LocalWorkspace直接引用，旧Workspace保留兼容导出；未复制页面状态或改变筛选/容量/角色规则。4个Workspace/FilesPage测试185905Z-9fb66011、tsc185910Z-70590d99、构建185927Z-ecd05e0d通过；独立端口Finder、本机CLI view真实fixture、兼容chunk按需加载三浏览器例通过 `temp/integration-test-runs/20261002T190014Z-ee27f2b2/`。这是依赖解耦，不是新FileBrowser/Home视觉替换，仍不满足F02.2全部退出条件。

## 治理上下文与延迟表单分离

新增governance-context.tsx承载GovernanceState、读取hook、mode与上下文栏；旧Governance保留兼容导出，管理面板和账号恢复按需import并有Suspense。未复制治理领域规则，CSS保持壳覆盖前静态加载；启用governance的登录页仍按需显示原恢复能力，不隐藏业务。tsc185103Z-45691aab、8个governance/AuthGate用例185120Z-06867330、构建185125Z-1fd4368b通过，产物含独立Governance chunk。独立team fixture治理/团队五例通过 `temp/integration-test-runs/20261002T185211Z-2dde229d/`。主Finder/Workspace和旧CSS退出仍未完成，F02.2保持未勾。

## 用户目录兼容模块按需加载

Workspace中UserDirectory改为lazy+局部Suspense，文件首页不再静态拉入用户管理代码；该模块无独立CSS副作用。tsc184107Z-6dcdfaad、构建184125Z-3e8dce73通过；独立team fixture资源请求断言初始不加载UserDirectory、进入受权目录后加载，以及治理深链/七宽度两用例共三例通过 `temp/integration-test-runs/20261002T184251Z-787f420f/`。未改变角色/tenant规则，Governance拆分与旧Finder/Workspace替换仍未完成。

## F02.2退出条件复核与兼容页按需加载

复读personal-workspace-rebuild文件处置表：F02.2要求替换旧Finder/Workspace并退出四份旧布局依赖，现有组件包装/局部清理不满足，保持未完成。当前Teams/KeyDirectory改为React.lazy，公共teamURL提取为无UI依赖模块，保留Teams旧导出；业务区域Suspense提供加载状态。

初次lazy使teams.css延迟覆盖既有布局，导航遮挡编辑按钮；失败证据183253Z-903e5e36保留。修复为兼容CSS在壳覆盖前静态加载，仅JS延迟，不强制点击/加超时。此前外层中断182259Z-cc3330b3未生成完整日志，不记验证；31948启动失败183202Z-b6e6f5f2为环境失败。使用独立端口避免共享fixture冲突，不终止归属不明进程。构建183504Z-d6008971，团队/治理五例183554Z-4618fe52通过；资源请求直接断言初始不加载Teams/KeyDirectory JS、打开对应页才加载，连同旧Key登录三例通过 `temp/integration-test-runs/20261002T183808Z-be0c9386/`。Governance/UserDirectory延迟加载及新FileBrowser/Home替换仍待推进，不据此勾F02.2。

## 搜索游标过期恢复

收到cursor_expired或分页HTTP410后清游标栈/页码/旧结果，保留输入并提示显式从第一页检索，不自动重放。11个SearchCenter用例181037Z-eccd89b9、tsc181041Z-e2668ea8通过；覆盖结构化code与纯文本410两类，确认第三次请求保留条件且无cursor。随后构建181336Z-80916317通过；分页410注入、保留条件、清结果、显式无cursor重查及coverage两浏览器例通过 `temp/integration-test-runs/20261002T181421Z-3f63d669/`。响应为网络替身，不作为真实索引游标失效集成证据。

## 精确搜索上一页与页码

成功读取后维护本次查询游标栈与页码，上一页重用已访问cursor，条件/范围变化清栈；不缓存文件内容，不自行构造cursor。9个搜索组件用例180609Z-917c2c6c、tsc180613Z-435b83a1、构建180631Z-197bd734通过；生产浮层下一页→上一页→新查询与覆盖两例通过 `temp/integration-test-runs/20261002T180720Z-53911837/`。查询为网络替身；游标过期恢复仍需实现，不登记完整搜索验收。

## 精确搜索后续页入口

SearchCenter保留exact next_cursor，明确本页非全部并提供下一页；仅提交服务端不透明游标，结果替换而非全量追加，条件/范围变更清游标。19个search/consumer测试175850Z-cc744ff2、tsc175856Z-c60d2d76通过，验证第一页→第二页原cursor、末页无按钮、新查询不带旧cursor。随后构建180029Z-e2382a8a通过；生产浮层下一页提交原游标、替换首批结果、改条件后无旧游标，及覆盖状态两浏览器例通过 `temp/integration-test-runs/20261002T180212Z-48ba4161/`。查询为网络替身；上一页/游标失效恢复及完整搜索验收仍待实现。

## 保存搜索条件提交保护

SearchCenter保存期间使用同步pending门与禁用按钮阻止重复提交；条件revision/context revision和mounted检查隔离旧成功/错误提示。条件改变或关闭界面不代表撤销已发送的服务端写入，不自动重试。18个search/consumer测试175258Z-72135d79、tsc175304Z-8ef7d930、构建175350Z-88457871通过；搜索覆盖与既有consumer五浏览器回归通过 `temp/integration-test-runs/20261002T175450Z-20f070e7/`。重复点击/晚到保存结果直接证据为组件测试，不冒充saved-searches真实持久化验收。

## 搜索范围切换清理

SearchCenter监听context变化，取消旧查询并清显示结果/覆盖/条件/模态/错误和保存提示，恢复exact idle；不自动查询。17个search/consumer测试174611Z-a391db3b、tsc174617Z-2d62ac34通过，直接验证已显示旧file_id和查询文本在换tenant/space后清除、请求数不增加。随后Web全量79文件388例174819Z-3df58e25、构建174827Z-0d864775、搜索浮层/能力恢复/coverage/旧深链四浏览器例174915Z-6c48ca36通过。范围清理直接证据仍是组件注入，浏览器回归不冒充真实权限切换；服务端权限仍是最终裁决。

## 能力探测错误分类

probeFlowCapability仅将plain-text request_failed且HTTP404/405作为旧部署缺能力；401/403/5xx强制传播，429及其他request_failed也不再伪装unsupported。26个client/consumer测试173943Z-13dbaf6e和tsc173949Z-4740124f通过，新增401/403/429/500/503纯文本负例。随后构建174206Z-8396dd69、生产搜索浮层503错误→显式重试恢复以及覆盖状态两浏览器例通过 `temp/integration-test-runs/20261002T174255Z-739fac0f/`。错误和恢复为网络替身，不改变后端能力合同。

## 精确查询覆盖状态保真

SearchCenter保留服务端exact coverage，显式展示；仅complete+零items可显示完整零结果，partial/未知改为已覆盖范围未返回并提示不能断言无文件。16个搜索/consumer用例173300Z-1f68b032、tsc173305Z-06774352通过，覆盖complete/partial/缺省。随后构建173600Z-e7564495、生产浮层complete/partial/未知三态浏览器用例通过 `temp/integration-test-runs/20261002T173648Z-4f2ed953/`，验证首次打开不发查询、三次显式提交及空态区分。查询和能力响应为明确网络替身，不把覆盖提示当作索引能力新增。

## 搜索条件变化失效旧查询

SearchCenter条件文本/口径/模态变化取消旧请求，清结果/覆盖信息/旧保存提示并回idle，不自动发搜索请求。14个search/consumer用例172631Z-1c5f03a7、tsc172636Z-a871fbea通过；文本与口径切换后忽略abort的旧成功响应不得显示，且请求数仍为一次。随后构建172825Z-62b9e824与consumer/浮层五浏览器例通过 `temp/integration-test-runs/20261002T172911Z-61b5fb40/`；晚到响应失效仍由受控组件测试直接验证，浏览器不冒充该故障注入。

## 搜索未查询与加载状态

SearchCenter增加idle/loading可读status和aria-busy，不把未提交或查询中当作零结果。11个搜索/consumer用例171928Z-5af12fce、tsc171933Z-d1aa9eb3通过；断言idle不请求、loading不呈现empty，保留原ready零结果分支。随后新增完成零结果与失败保输入的受控响应测试，两例172232Z-ce7df170通过；构建172236Z-9874235a、浮层与原consumer流程五浏览器例通过 `temp/integration-test-runs/20261002T172324Z-398012b6/`。状态转换注入为组件证据，浏览器保持既有替身边界，不冒充真实索引验收。

## 搜索浮层查询取消保护

SearchCenter复用latestRequest并将AbortSignal传给原queryFiles/queryContent；卸载取消，最新请求与context revision同时保护结果/错误写回，精确查询成功清除旧content元信息。11个search/consumer用例171359Z-9bd95145、tsc171404Z-d45b844f通过，新增关闭后晚到失败的负例。随后构建171547Z-e95f71b3、搜索浮层/旧检索与consumer流程六浏览器例通过 `temp/integration-test-runs/20261002T171639Z-5b0a28fe/`；取消晚到请求仍以组件负例为证据，既有consumer网络替身范围不变，不宣称完整搜索状态验收。

## CommandSearch壳层入口

新增app/CommandSearch，顶栏按钮及Ctrl/Cmd+K打开现有AppDialog/SearchCenter，按需探测content_query，失败可重试且卸载取消；输入/编辑/选择文本/已有dialog时快捷键让位。Escape/关闭回原触发焦点，页/空间/身份变化关闭。tsc170751Z-13282554、构建170808Z-ed27e836、搜索浮层键盘/手机/焦点、旧搜索深链和断点三浏览器例通过 `temp/integration-test-runs/20261002T170854Z-163cac6d/`。仅壳层入口与现有查询界面复用，未验收查询结果返回滚动/持久草稿或完整S3搜索状态，不用原型瞬态替代业务结果。

## 手动复查与恢复鉴权共用

提取verifySessionIdentity只读检查，手动“重新检查访问权限”先校验user/instance再读成对投影，防止旧壳显示新会话身份的数据；不覆盖CSRF。16个session/poll测试165910Z-d0b97861及tsc165913Z-8eda312c通过，包含身份匹配/换用户/换实例及CSRF不变。随后构建170100Z-fa5f89d4通过；生产浏览器显式将/me用户替换为另一身份，复查保持拒绝且零任务GET，移除替身后原会话复查恢复，以及前台鉴权两例通过 `temp/integration-test-runs/20261002T170237Z-584dbe5c/`。替身未修改真实用户或会话凭据。

## 前台恢复读取鉴权门

poll增加beforeResume只读门，后台→前台下一tick先GET /me，成功且user/instance与当前身份一致才读任务；401/403或身份变化停止并清投影。初次会话仍走原loadSession，不从此检查覆盖CSRF或身份；取消的旧检查不释放新一代读取。13个poll/session用例165240Z-cb1008d6、tsc165243Z-f4b4c40c通过，包含鉴权未完成不读取及拒绝后零读取。随后构建165519Z-f2eed5e0、浏览器前台恢复先GET /me后读任务、注入/me 401后不再读任务且清旧记录，以及Retry-After两例通过 `temp/integration-test-runs/20261002T165601Z-fe989a65/`；可见性与拒绝为显式模拟，不据此完成F11.1全任务。

## 活动与空闲任务轮询间隔

poll支持动态正常间隔，接收成功投影后再计算下一次间隔；WorkspaceApp基于operations/inbox已加载状态选择活动2秒/空闲30秒，未知/失败/中断按仍需观察处理。状态类别变化重新调度，失败退避/后台暂停/失权门保留。8个定向测试164853Z-3cec5d18、tsc164610Z-921a1047、构建164628Z-68236917通过；生产可见性/Retry-After/失权三个浏览器例通过 `temp/integration-test-runs/20261002T164715Z-394fc3e8/`。动态30秒调度为fake-timer单元证据，完整前台重鉴权仍未实现，F11.1不提前完成。

## Retry-After异常数值保护

拒绝负数/带正号/小数秒数回退到Date.parse，避免宽松日期解析将非法数值变为等待日期；非有限溢出继续忽略。14个retry/poll定向用例通过 `temp/integration-test-runs/20261002T164114Z-cb52a7ef/`，包括epoch零基准的反例；不重复运行本轮无变化的浏览器布局测试。

## 读取失败退避与Retry-After

transport解析Retry-After秒数/HTTP日期并保留到错误；poll失败使用2/4/8/16/30秒基准加最多10%正抖动，成功恢复正常间隔，后台往返保留notBefore下限。长延时分段计时避免setTimeout溢出。成对投影失败合并两侧最长Retry-After，鉴权错误仍优先停机。最新15个poll/retry/projection测试163430Z-bbd80fd2、tsc163433Z-ca1fbf4a通过；早一轮含transport的18例163239Z-fd4f9734通过。随后构建163711Z-ed235d8b通过；生产浏览器503+Retry-After:6替身计时验证保留旧任务、4.2秒观察期无额外GET、恢复读取距失败≥6秒，与失权停止/显式复查两例通过 `temp/integration-test-runs/20261002T163753Z-d961c625/`。故障与授权变化为网络替身，不宣称完整F11.1或后端限流验收完成。

## 页面渲染故障隔离

新增app/RouteBoundary包围业务页面区域，导航/账户/反馈不随子页面渲染错误卸载；安全中文提示不显示原异常，显式重试或路由/身份范围变化重置，提示未提交输入可能需重填。三个边界/壳层用例162339Z-cb4b75be、tsc162342Z-9f2ab192、构建162358Z-53ed78db通过；Finder/收件深链/手机三浏览器例通过 `temp/integration-test-runs/20261002T162442Z-586e5c66/`。随后生产浏览器通过 `temp/integration-test-runs/20261002T162709Z-cd62f9cc/`：/agents显式返回scopes=null替身触发真实渲染错误，边界显示安全提示、侧栏/账户仍可用、切文件页正常；恢复GET数据后重进Agent显示正常空态。事件/异步API错误仍由原页面负责，不声称所有异常都能捕获。

## 失权后显式复查入口

失权状态进入poll effect依赖和启动门，任务/收件换页不重建自动读取；增加“重新检查访问权限”只读按钮，成功读回成对投影才清拒绝状态。tsc161739Z-48f0cf82、构建161755Z-43f59003通过；浏览器注入403、换页超过4秒无新增GET、显式复查后恢复列表通过 `temp/integration-test-runs/20261002T161854Z-d6b0ed40/`。授权变化为网络替身，未宣称后端撤权集成或完整前台重鉴权完成。

## 任务失权与空态区分

增加taskAccessDenied投影状态；失权后任务/收件持续显示不可访问说明，隐藏历史工具与空态，只有成对成功读取后清除拒绝状态，不依赖自动消失toast。tsc161237Z-451dec84、构建161254Z-b2147f05、真实历史与注入403两个浏览器例通过 `temp/integration-test-runs/20261002T161340Z-bbc4feeb/`；断言任务转收件不显示零记录或旧文件名。401/403为前端替身证据，未完成跨页面重鉴权/完整任务策略。

## 任务失权投影清理

poll鉴权失败回调清空operations/inbox并显示安全提示；普通临时失败不清列表。12个poll/projection测试160633Z-d5ab3f0e、tsc160635Z-d76da28a、构建160749Z-69dfa62f通过。生产页面注入403后旧任务名称消失、自动读取停止、可见性变化不重启，以及后台暂停两浏览器例通过 `temp/integration-test-runs/20261002T160853Z-0fe39a8e/`。403为明确网络替身，不代替后端撤权验收；页面切换后的重新鉴权及完整退避仍待实现。

## 鉴权失败停止轮询

transport错误增加HTTP status供调用方判定，保留原message合同；poll对401/403或unauthorized/forbidden停止本次调度，visibilitychange不恢复，已取消旧请求不能停止新一代。成对读取优先传播鉴权错误，避免被另一侧503遮蔽；覆盖纯文本401/403。16个poll/projection/transport用例 `temp/integration-test-runs/20261002T160303Z-8f6b41b0/`、tsc160306Z-ac87cfad通过。本片尚未生产构建/浏览器验证；原投影的失权清理、前台重鉴权与完整退避策略仍待实现，不宣称F11.1完成。

## 正文gutter规格对齐

新增workspace-gutter=16px/mobile-gutter=12px主题值；普通content移除旧24/18px水平覆盖，团队兼容独立布局保留。shell-breakpoint新增767/768/1440实际padding断言，构建155713Z-0eb405c3、详情/手机/断点/reflow六浏览器例通过 `temp/integration-test-runs/20261002T155753Z-d3e56694/`。仅正文基础间距对齐，不代表全部文件行密度/字号已达到规格。

## 正式主题数值对齐

按UI Spec§1将浅画布改为#f5f6f4、深边框#587563、控件/面板/工作区圆角6/8/12px；ui-radius明确8px，不再依赖旧14px根字号换算。主题单一来源测试新增精确值断言，155241Z-f9a78ddf通过；构建155244Z-4ef2a297、浅深侧栏/手机/详情与断点八浏览器例通过 `temp/integration-test-runs/20261002T155331Z-e2ecacdd/`。其它字号、正文gutter、详情类型和搜索浮层仍需按规格核对，不能据token修改勾F03.1完成。

## 侧栏账户与容量区域迁移

新增app/SidebarFooter，容量仍复用capacityOf/CapacityBar，未知占用和配额诚实显示；账户菜单原设置/登出回调不变。tsc154656Z-39fd0e5c、7个footer/capacity用例154754Z-be05bc1d、构建154758Z-d2b47be1通过。真实本地登出失败/成功、导航宽度及浅深色四浏览器例通过 `temp/integration-test-runs/20261002T154840Z-9f1576a3/`。本轮仅迁移壳层视图，未完成F02.2全部退出条件。

## 手机导航抽屉编排迁移

新增app/MobileNavigationDrawer，复用AppSheet，把分组/受权菜单/选择关闭与登出回调移出WorkspaceApp；不复制权限、不改焦点原语。tsc154205Z-ecaaaaed、三个组件例154309Z-6b7e0aee、构建154313Z-52634b84通过；手机完整菜单/模拟触控/键盘编辑让路三浏览器例通过 `temp/integration-test-runs/20261002T154400Z-b75521d6/`。账户容量和其他壳编排仍未全部迁移，不勾F02.2。

## 桌面导航编排迁移

新增app/DesktopNavigation，从WorkspaceApp移出三组渲染、折叠摘要/计数和当前页按钮；接收原应用层过滤后的groups，不复制权限规则。计数沿用pendingNavigationTasks已加载去重投影。6个组件/导航用例153819Z-6c611486、tsc153822Z-237e9fa6、构建153840Z-849ffbf1通过；240/68宽度、分组计数、浅深可读性四浏览器例通过 `temp/integration-test-runs/20261002T153923Z-51533443/`。手机完整菜单和账户容量外壳仍由原组件持有，未勾F02.2完成。

## 恢复按钮与有效选择一致

TrashPage批量按钮使用父层restorableSelection产生的有效数量，不再用原始picked启用；单项恢复在busy时禁用。五个trash测试153255Z-f7594980、tsc153259Z-f9802e9e、构建153336Z-393d00a5通过，真实本地双空间回收恢复浏览器通过 `temp/integration-test-runs/20261002T153420Z-4e014b2c/`。仅防止无效/处理中点击，不替代服务端幂等和权限检查。

## 无消费者旧用户表格规则移除

确认生产TS/TSX无user-table类，UserDirectory已使用统一DataTable；删除workspace-layout中旧.user-table基础与1100/600响应式规则，保留实际使用的user-filters/user-key-list。构建152754Z-6f5ced1e、独立team fixture治理深链及用户目录/权限矩阵/存储表单浅深七宽度两用例通过 `temp/integration-test-runs/20261002T152838Z-af3e3d26/`。没有修改表格行为、权限或断言，F02.2整体验收仍未登记。

## 团队组件层覆盖收敛

shadcn.css两层团队覆盖移入app/team-compat.css，合并原最终值并加:where(.teams-workspace)作用域；删除原拼错team-workspace且不生效规则，不擅自激活。静态151950Z-54ff1a0f、构建151952Z-f4e099e2通过。团队三例在152039Z-a13fa947通过，但compact治理测试390px误点旧桌面侧栏导致超时；改成按视口使用真实手机菜单/桌面入口，不放宽间距/宽度断言。重跑compact两例通过 `temp/integration-test-runs/20261002T152406Z-fccea7b7/`，覆盖浅深主题七宽度和治理隔离，失败证据保留。

## 团队管理布局局部隔离

workspace-layout.css内28处team-header/layout/nav/body选择器增加:where(.teams-workspace)祖先，保持原优先级，避免管理布局规则匹配其它页面。UI静态151533Z-fc36a0b3、构建151550Z-db97fd55通过。显式启用独立本地team fixture后团队/账号激活/成员/网盘托管/角色导航四浏览器例通过 `temp/integration-test-runs/20261002T151632Z-150a6b0e/`，无skip；此前普通fixture中跳过的workspace-access本轮已有执行证据。不外推所有管理CSS均已隔离或真实生产权限验收。

## 轮询成对读取失败收敛

新增readOperationProjection：operations/inbox用allSettled等待两侧结束，一侧失败不提前安排下轮，不提交一新一旧投影；传入相同AbortSignal且最终重验取消。8个projection/poll测试通过 `temp/integration-test-runs/20261002T150957Z-4a34c4ab/`，tsc151000Z-04ca8076通过。覆盖任一侧早失败、另一侧滞后、正常顺序和取消后晚到；随后构建151137Z-84722745、历史真实本地链与可见性模拟两浏览器例151223Z-2c5375ba通过。失败成对收敛仍以单元注入为证据；未实现完整F11.1轮询策略。

## 任务轮询后台暂停

poll订阅visibilitychange：隐藏清timer并abort；显示使用新controller按原间隔恢复，晚到旧响应/旧finally不写状态、不创建额外timer；stop移除监听。8个poll/transport测试 `temp/integration-test-runs/20261002T150317Z-dd9d24f4/` 和tsc150319Z-248c2b8b通过，包含初始隐藏、往返、忽略abort的旧响应、退出后无timer。随后生产构建150531Z-a4f2b5b1通过；task-poll-visibility浏览器用例通过 `temp/integration-test-runs/20261002T150616Z-4002a73f/`，在生产页面显式模拟document.hidden并用GET替身计数，隐藏超过4秒无新增读取、恢复后重新读取且无写操作。可见性为模拟，不宣称实际后台省电或前台重鉴权完成。

## 原任务/收件轮询无重叠保护

旧setInterval改为read完成后再setTimeout，卸载取消timer和AbortSignal，并fence忽略abort的晚到响应；失败保留原投影，重试仅GET。沿用旧4秒间隔，尚未实现F11.1活动/空闲/后台暂停/重鉴权/Retry-After全策略。7个poll/transport测试145854Z-42630ae5、tsc145857Z-6cb5fac1、构建145930Z-638e5ddc通过；真实本地history与加载任务计数两浏览器例通过 `temp/integration-test-runs/20261002T150012Z-c8d52fbd/`。非重叠及晚到响应为可控单元证据，不外推完整任务中心验收。

## 文件刷新最新请求隔离

文件列表refreshFiles改为每次取消前请求并检查请求身份，配合原space/page及transport context revision，避免A→B→A时早期A响应覆盖新A。effect退出取消，无效范围也撤销前请求；过期错误不写当前error。11个定向测试145449Z-0bff1e59、tsc145454Z-0396410b、构建145510Z-768ea782通过；Finder与真实双空间回收恢复两浏览器例通过 `temp/integration-test-runs/20261002T145553Z-ed411c49/`。乱序返回为单元级可控Promise证据，浏览器验证现有真实链未退化，不冒充已注入浏览器网络乱序。

## 回收站选择范围保护

空间/page/身份变化清空旧picked；批量恢复前按当前已加载files、space_id、trashed和选择交集重验，去重且空集不发请求。展示列表也排除旧空间返回。4个定向测试144751Z-7d98936c、tsc144755Z-c29e6c98、构建144834Z-af1d00cb通过；真实本地fixture回收恢复通过 `temp/integration-test-runs/20261002T144917Z-671fb717/`。随后扩展真实浏览器双空间用例：通过本地API创建第二可丢弃空间，勾选后切换空间批量恢复禁用、旧文件消失；切回选择不复活，重新全选后恢复并在原网盘回读两文件。通过 `temp/integration-test-runs/20261002T145125Z-2849e145/`，无网络替身；服务端权限仍是最终裁决。

## 退出旧全局侧栏读取器

移除已无生产引用的initialSidebarCollapsed全局键读取路径，将sidebar-preference改成useSidebarPreference实际复用的readSidebarPreference与TABLET_QUERY。测试改为真实scoped读取，保留默认/显式/非法/存储异常并新增匿名不读；九例144403Z-13b8bbb9、tsc144406Z-6ee109c1通过。没有保留两个语义相反的读取器，也不删除用户旧存储。

## 侧栏基础规则去重

将最终侧栏padding=24px 16px 16px和backdrop-filter=none收回style.css基础规则，移除finder.css两处重复.sidebar宽度/背景/边框/padding定义；折叠/媒体规则保持。构建143950Z-8aea4fb0、浅深可读性/240-68宽度/767-768边界四浏览器例通过 `temp/integration-test-runs/20261002T144032Z-aed38a9f/`。未宣称全量CSS隔离完成。

## 旧服务端身份不完整的历史偏好

historyScope在instance或user缺失时返回空scope；History可临时隐藏/撤销但不读写共享存储，完整身份到达后重载。9个history/tasks用例143654Z-8f0dbef4及tsc143658Z-8aa4a71c通过，避免旧服务端无instance_id导致偏好串用。本片仅前端兼容与focused验证。

## 历史隐藏记录作用域隔离

任务/收件History scope补全instance、user、tenant、space、page并逐段编码；内部ScopedHistory按scope重挂，清理筛选、确认与撤销状态，读取新身份隐藏列表。不迁入归属不明确的旧键、不删除服务器Operation。8个history/tasks测试通过 `temp/integration-test-runs/20261002T143144Z-a819699c/`，tsc143149Z-922bd2fd通过。覆盖同挂载实例切换与原偏好恢复、分隔符歧义；随后生产构建143338Z-654f0d4e通过；history-i18n真实本地上传产生记录、清理刷新隐藏/恢复显示、收件与传输固定版本预览用例通过 `temp/integration-test-runs/20261002T143420Z-b45183a3/`。其中错误本地化分支为既有404替身，不计真实权限拒绝；不据此声称所有本地偏好已隔离。

## 独立本机视图样式兼容保护

发现insights/main.tsx独立加载style/finder而不经过workspace-layout，未直接强行换成Web壳入口。对CSS整理后的当前dist执行真实CLI本地fixture：私有启动会话消费后清URL、离线owner只读、15字节用量回读、写入口403、在线owner目录与固定引用复制；首次142656Z-20354327通过。补手机390px文档无横向溢出断言后再次通过 `temp/integration-test-runs/20261002T142743Z-3aa2ea4e/`。仅临时alias目录、pure-Go构建、无真实云或永久删除；不是完整CLI/MCP兼容验收。

## 全量单测与主内容级联收敛

本轮组合验证：68测试文件335用例通过 `temp/integration-test-runs/20261002T142208Z-3bf7e64d/`。随后将.content最终基础布局集中到workspace-layout.css，移除style/finder中被覆盖和重复声明，保留drive/team局部及媒体规则。UI静态142356Z-99ddbf1e、构建142358Z-54a3264b、断点/reflow/详情/真实本地回收恢复六浏览器例通过 `temp/integration-test-runs/20261002T142440Z-ff5b89d4/`。仍不宣称旧CSS全部清空或F02.2整体完成。

## 壳层手机/平板断点对齐

workspace-layout顶栏及其同块内容布局由旧760/761分界改为767/768，与手机导航一致；平板默认范围768–1099。新增767→768→767浏览器测试，验证手机无sidebar、动作换行、10/16px顶栏padding及平板68px，无刷新切换。构建142040Z-70e68605、边界与导航宽度两例通过 `temp/integration-test-runs/20261002T142120Z-30f384c2/`。其它业务面自有窄屏断点未批量替换，不宣称所有旧响应式CSS已清理。

## 顶栏级联重复清理

删除finder.css重复app背景、主容器margin/width以及被覆盖的顶栏gap/padding；sticky/top/z-index/blur/background移入workspace-layout.css已有顶栏规则，语义token注释更新为实际来源。不新增覆盖层。UI静态141744Z-b40a903f、构建141747Z-7b4b0c14及桌面尺寸/手机/reflow/Finder四浏览器例通过 `temp/integration-test-runs/20261002T141826Z-daf1be97/`。仅确认本批级联保持，旧CSS尚未全部退出生产入口。

## 页面迁移后旧壳残留清理

用tsc --noUnusedLocals --noUnusedParameters定位WorkspaceApp失效导入、preview/textPreview及仅写不读driveParent状态，定向删除；OneDrive items请求仍使用原i.id，不改变目录参数。诊断前141217Z-b1081b3f、后141356Z-fd9c51c0：后者已无WorkspaceApp未使用项，但其它模块仍有问题，严格未使用检查整体不记通过。常规tsc141413Z-03f46dcd和features/上传恢复22例141443Z-0ad6e8cc通过；未为清门修改其它模块，未重新声称生产浏览器已验证这一清理片。

## 回收站页面迁移

新增features/trash/TrashPage，工具栏、选择、列表/网格、空态从WorkspaceApp迁出，删除内部永不成立的非trash分支；保留原空间/目录筛选和保留期/恢复API回调，永久删除无回调且保持禁用。三个组件例140934Z-94aa8802覆盖原文件身份恢复、受限身份禁用及搜索空态；tsc140938Z-f17628b1、UI静态140955Z-b95e5280、构建140957Z-e18bf5d1通过。真实本地fixture上传/软删除/批量恢复回读及Finder不覆盖两个浏览器例通过 `temp/integration-test-runs/20261002T141042Z-38e0c1b6/`。未运行或启用永久删除；F02.2旧CSS与壳内其余编排仍待清理。

## 永久删除不可达残留清理

移除旧壳purge/purge-batch确认表单、占用查询effect、proof/usage状态和标题映射，不留可重新接线的永久删除提交器。断开连接/撤销授权保留独立确认。tsc140407Z-0dcff4c3、构建140424Z-d698b1ca、回收恢复与两类确认三浏览器例通过 `temp/integration-test-runs/20261002T140503Z-9e42207e/`。无永久删除请求或真实数据操作；仍不修改服务端兼容/保留策略。

## 回收站手动永久删除边界

按本目标禁令关闭旧WorkspaceApp单项/批量永久删除按钮（显示未启用原因），Shift+Delete仅提示不可用；移除该壳内批量purge和确认永久DELETE请求，旧确认分支不可提交。恢复入口不变，不删除文件或后端兼容合同。tsc140016Z-47d95d80、构建140033Z-cef9a6e1通过；本地真实fixture上传→软删除→永久按钮禁用→批量恢复回读，以及连接/Agent确认边界三个浏览器例通过 `temp/integration-test-runs/20261002T140113Z-44b0af5a/`。未更改服务端保留策略，未执行永久删除；其它产品面安全门仍需最终逐项核对。

## 侧栏动态断点默认值

useSidebarPreference订阅平板媒体查询；无显式偏好时桌面/平板不刷新自动240/68切换，显式选择与身份隔离保持，视口变化不写localStorage。6个hook测试135310Z-d664dc03、tsc135313Z-db52d369、构建135436Z-bec6929c通过。浏览器首次135519Z-c8b68748暴露测试登录后过早读取/me，补等待已登录app与200断言；保留失败记录，不延长超时。无刷新断点及显式偏好/键盘两例通过 `temp/integration-test-runs/20261002T135558Z-79ee43ce/`。

## 文件页面三态编排迁移

新增features/files/FilesPage，将DriveHome/不可访问网盘/Finder三态合并为单入口；复用原Finder，固定focus链接、可用OneDrive连接过滤、允许空间跳转和业务回调保留。不挂载无权限Finder，不建全局文件store。8个定向测试134903Z-4c8b9cb8、tsc134906Z-4ab6ba16、构建134922Z-c6a0e5bc通过。Finder本地文件/目录/预览、原fixture分享兼容及搜索三个浏览器例通过 `temp/integration-test-runs/20261002T135015Z-6e58f0fe/`；既有分享用例仅本地fixture、不启用真实公共分享。S1完整发布恢复和F02.2旧回收站/CSS移除仍未全部验收。

## 旧收件投影页面迁移

新增features/tasks/LegacyInboxPage，保留原History和Operation固定版本投影；回调预览/引用，恢复入口统一navigatePage，父层继续处理API与版本读取。组件不建submission审核状态。任务页五例134230Z-ca57fffe、tsc134235Z-a5041a75、UI静态134251Z-f73962b2通过；生产构建134305Z-5f6a3289，收件深链/登录刷新/手机返回和旧页面入口两浏览器例通过 `temp/integration-test-runs/20261002T134345Z-6eedc615/`。不证明新版收件接收/拒绝/归档生命周期完成，F02.2仍需旧文件页及布局清理。

## 原操作传输页面迁移

提取features/tasks/LegacyTransfersPage，History、表格、进度、结果与取消确认从WorkspaceApp移出；原Operation对象回调恢复/取消/固定引用，AbortController、原API与文件重选继续由父层持有。交付tab改走已有navigatePage，刷新深链一致。8个定向测试133735Z-228eb282证明发送100%不显示发布结果、确认前不取消、原ID和固定版本不替换；测试误用Playwright exact参数导致tsc失败，移除后tsc133815Z-5ce0ffc7通过，构建133832Z-5abb8e82、六浏览器回归133912Z-3cbd5107通过。后者包含现有consumer替身链，不冒充原操作真实断网重启验收；完整新任务投影仍未完成。

## 存储连接页面迁移

提取features/storage/ConnectionsPage，卡片/状态/Provider能力说明及动作入口从WorkspaceApp移出；原连接验证、OneDrive items读取、创建空间及断开确认仍由父层原API处理。组件只回调原Connection，不持凭据或请求Provider。7个定向测试132852Z-42874537覆盖S3/OneDrive动作、disabled、platform不可断开和空列表新增入口；tsc132855Z-03833659、UI静态132911Z-3f406315通过。LSP两文件确认无错误、一文件未确认；随后生产构建133140Z-a6f3e74b通过。连接列表使用显式替身的桌面/手机、disabled/platform动作限制、断开确认取消零写请求与旧页面入口两浏览器例通过 `temp/integration-test-runs/20261002T133220Z-b66c3742/`。同次workspace-access因既有isolated team fixture条件跳过，不记通过；不证明真实Provider验证或团队权限全链，不据此勾完整F02.2或S4存储验收。

## 导航分组偏好身份隔离

read/writeNavGroups通过显式instance+user键读写；新增useNavGroups切换身份时恢复自身值，匿名不写，旧全局键保留不迁入。读取仅接受三组有效布尔字段，损坏数据回默认。7个定向测试132227Z-a1c5c1ec、tsc132230Z-8431ef37、构建132316Z-8143523e通过。导航宽度测试新增折叠后刷新仍折叠，连同手机导航两浏览器用例通过 `temp/integration-test-runs/20261002T132353Z-77690287/`。这些偏好不承载权限或业务状态，F02.2/F03整体验收仍未完成。

## 侧栏偏好身份隔离

生产侧栏改用useSidebarPreference，按instance+user编码键保存；仅显式切换写入，匿名不写，身份切换恢复自身偏好。旧全局drivebridge:sidebar-collapsed不迁入未知归属用户；旧键不删除。三个组件测试覆盖用户/实例隔离、匿名默认与存储异常，131625Z-8b22f919；tsc131628Z-0bbbe4a8、构建131841Z-29dca7ab通过。真实fixture会话下宽度/刷新/平板默认/键盘/手机不覆盖偏好两个浏览器例通过 `temp/integration-test-runs/20261002T131918Z-624657cc/`。分组折叠等其它偏好尚未完成隔离，不据本项宣称全部偏好已迁移。

## 工作树与证据

2026-10-02 只读记录：HEAD 为 `73609bfca42093968f1eee0eb8188d3624e4e970`，证据 `temp/integration-test-runs/20261002T062641Z-309459a8/`。逐文件 dirty/untracked 清单保存在 `temp/integration-test-runs/20261002T062641Z-6463ef91/stdout.log`。该清单是状态证据，不是源文件备份；不得用 HEAD 恢复这些文件。所有清单条目当前保护去向均为原路径保留，迁移时逐文件读取后再定向修改，不清理未跟踪文件。

当前生产 Vitest 基线为 40 个测试文件、228 个用例通过：`temp/integration-test-runs/20261002T062704Z-5e417572/`。定向 API/路由/阅读器/存储基线为 5 文件、20 用例通过：`temp/integration-test-runs/20261002T062625Z-943bceb7/`。这些是本地测试替身下的验证，不证明服务端真实持久化或浏览器视觉效果。

## 旧壳截图与浏览器基线

当前生产构建通过：`temp/integration-test-runs/20261002T063254Z-4eaa7c68/`。保留大于500kB chunk的构建警告，不称性能验收。

复用现有 `workspace-design.spec.ts`、`unified-login.spec.ts`，两个浏览器测试通过：`temp/integration-test-runs/20261002T063334Z-72c409aa/`。测试使用本地可丢弃HTTP fixture，覆盖登录提示、创建网盘、目录树/侧栏折叠、旧analytics/users深链与手机网盘入口。五张截图保存在该run的artifacts中，不包含输入凭据。

已查看 `green-drive-home.png`（1440×1000）与 `green-drive-mobile.png`（390×844）：旧桌面三组导航、网盘列表及详情可见；手机使用窄图标栏，内容未见横向溢出，但搜索占位文字明显截短。这是重建前事实记录，不是冻结v39新壳视觉验收。另保留dashboard、key-directory与collapsed-rail截图。

## 迁移保护边界

| 当前文件/目录 | 已知职责 | 重建处理 |
| --- | --- | --- |
| `web/src/main.tsx` | App、登录加载、hash 监听、ScopedApp、share 入口、生产挂载 | 先提取后替换；不一次性重写或删除 |
| `web/src/api.ts` | 会话 CSRF、上下文 revision、同源 fetch、上传原 Operation 恢复 | 保留行为及调用签名；用测试保护后再提取 |
| `web/src/workspace-route.ts` | hash/旧 URL 解析 | 保留别名及 tenant/space 语义 |
| `web/src/PreviewFiles.tsx`、`PdfPreview.tsx`、`SpreadsheetPreview.tsx`、`html-preview.ts` | 安全预览及固定版本获取 | 复用阅读器；不从原型替换为模拟内容 |
| `web/src/Governance.tsx`、`Teams.tsx`、`UserDirectory.tsx`、`KeyDirectory.tsx` | 既有治理与身份管理入口 | 保留授权条件及局部样式，不能因个人壳重建丢失 |
| `web/src/consumer-flows/`、`info-workspace/`、`insights/` | 原业务页面与独立统计入口 | 保留到新页面显式接管并通过回归 |
| `web/src/components/` | 既有 Base UI/shadcn 原语 | 复用，不新增并行组件库 |
| `web/prototypes/` | 原型参考与并发设计工作 | 原路径保留；生产不得导入模拟业务模型 |
| `web/tests/`、`web/public/`、配置及其余清单条目 | 浏览器用例、资产、构建和未知既有工作 | 默认原样保留，变更前逐项读取 |

## 当前入口事实

`main.tsx` 中的生产挂载按 `#share/` 选择既有 SharedView，否则经 AppearanceProvider、ScopedApp 进入 App。ScopedApp 在上下文事件后重建应用；不能在拆壳时删除这个隔离行为。存在旧分享入口不代表本目标授权启用公共分享。

API mutation 使用会话 CSRF 与 same-origin credentials；不增加浏览器长期 token 持久化。上传的 `checkContext` 同时检查 AbortSignal 与 dataContext revision，恢复身份与 digest 验证不得改成重新分配上传。

## 上传恢复客户端保护

新增 `web/src/upload-recovery.test.ts` 五个用例：原operation恢复不重新创建、digest不匹配零请求、恢复中切换上下文不调用complete、发布失败不报告completed、已完成/已中断不重启请求。与API/深链/安全阅读器定向验证合计21用例通过：`temp/integration-test-runs/20261002T063651Z-8cfa83df/`；新增文件无LSP错误。使用零字节File和网络替身验证控制流，不替代真实分片/存储发布验收。

## F02.1 首个等价提取

将tab-local context、revision、事件与header/URL投影提取至 `web/src/app/data-context.ts`；`api.ts` 保持旧导出并引用同一状态，不新增store、凭据或持久化。新增共享引用、重复设置仍递增revision、身份请求省略space和保留固定version/hash测试。与上传恢复/API/路由/治理合计24用例通过：`temp/integration-test-runs/20261002T063939Z-ee64592f/`；完整TypeScript检查通过 `temp/integration-test-runs/20261002T064016Z-01c65dc0/`。LSP两文件结果不确定，不用空诊断代替类型检查。只完成窄提取，AuthGate、typed clients与新壳尚未完成，F02.1不登记完成。

## F02.1 作用域页面隔离提取

生产main.tsx已改用 `app/ScopedWorkspace.tsx`，保持原hash解析、缺tenant时沿用当前tenant、context事件递增key并重挂页面的行为。页面作为ComponentType传入，不建立领域store。新增测试验证跨scope页面局部状态重置、初始managed深链、同scope不重置及卸载清理监听。定向26用例通过 `temp/integration-test-runs/20261002T064409Z-aa47d210/`；tsc通过 `temp/integration-test-runs/20261002T064412Z-a5afc787/`。未触碰旧分享分支或登录逻辑，仍需后续生产构建与浏览器回归；main.tsx既有未用变量/空catch告警不顺手改写。

## F02.1 会话读取提取

新增 `app/session.ts` 的SessionView/loadSession；main初始检查和密钥登录后刷新复用同一方法。请求仍通过既有api：GET /me省略space、带tenant和可选AbortSignal；成功后才设置内存CSRF，未授权或旧上下文响应不得覆盖CSRF。29个相关定向用例通过 `temp/integration-test-runs/20261002T064727Z-0048721a/`，tsc通过 `temp/integration-test-runs/20261002T064730Z-801fc832/`。未重构登录UI或改变错误处理，尚不等于完整AuthGate完成。

## 会话加载卸载保护

初始会话effect现在传入AbortController并在卸载时取消；已卸载的页面不再接收会话/登录模式结果、错误与loading回写。loadSession在写内存CSRF前显式检查取消，防御传输仍返回成功的情况。30个定向用例通过 `temp/integration-test-runs/20261002T065029Z-c53c0674/`，tsc通过 `temp/integration-test-runs/20261002T065033Z-183b8d72/`。LSP两个文件未确认，以实际类型检查为准。未声称其它页面所有异步请求都已可取消。

## 提取后浏览器验证与入口收敛

生产构建通过 `temp/integration-test-runs/20261002T065241Z-31b9850e/`（保留大chunk警告）。现有keylogin、unified-login、workspace-design四个浏览器用例通过 `temp/integration-test-runs/20261002T065319Z-d91de507/`：包括普通Key无管理员入口、登录/撤销、桌面与手机导航和旧深链。未创建新的公共分享。

Vite仍保留index.html和insights.html双入口；后者的 `src/insights/main.tsx` 保留本机会话、LocalWorkspace与本机统计页面，不受个人壳替换影响。`#share/` 旧分支原样保留，不改变产品禁止启用公共分享的边界。旧治理与团队入口由原组件和路由继续承载。所有dirty/untracked条目遵循上表原路径保留规则。

结合原40文件228用例基线、恢复控制流负例、安全HTML预览和实际浏览器权限负例，F01.1保护基线与F01.2 runner静态保护可以登记；不将这些测试冒称新S1/S2闭环验收。

## F02.1 Transport提取

新增 `app/transport.ts`，复用同源fetch、上下文revision检查、旧plain-text错误兼容与内存CSRF；api.ts仍re-export原api/setCSRF，上传/下载用同一csrfHeaders。session只以type引用UserIdentity，运行时直接依赖transport。新增共享状态、不可变header快照、错误兼容、signal/幂等header测试。31个定向用例通过 `temp/integration-test-runs/20261002T065736Z-baeae306/`，tsc通过 `temp/integration-test-runs/20261002T065740Z-bf18662e/`。本次未改变HTTP协议或持久化凭据。

## F02.1 登录表单提取

新增 `app/KeyLoginForm.tsx`，main继续拥有登录请求与会话更新，组件只接收受控输入与提交回调；保留password/autocomplete off/login-key标识，额外拒绝busy或空白输入的直接form事件。30个相关定向测试通过 `temp/integration-test-runs/20261002T070110Z-d01913d0/`，tsc通过 `temp/integration-test-runs/20261002T070113Z-de823f64/`。LSP两文件确认无错误，main结果未确认。尚未改变新壳导航或正式视觉。

## F02.1 旧路由别名类型化

workspace-route.ts集中维护六个旧别名与WorkspaceAlias/WorkspaceRoute类型；drive/governance/flows特殊解析不变，未知路径继续由shell白名单处理。新增逐别名tenant保留及prototype属性名不误匹配测试。31个路由/壳层用例通过 `temp/integration-test-runs/20261002T070412Z-7eb9f730/`；tsc通过 `temp/integration-test-runs/20261002T070416Z-f1dc7433/`。LSP一文件未确认，不以空诊断宣称clean。

## F02.1 页面导航reducer

新增 `app/navigation-state.ts` 的Page类型与纯reducer，生产App以同一state保存page/tab；hash路由一次dispatch原子更新，原单字段调用保持语义。文件选择和目录数据不进入reducer。33个壳层/路由用例通过 `temp/integration-test-runs/20261002T070828Z-0974404f/`，tsc通过 `temp/integration-test-runs/20261002T070832Z-94646e87/`。未改变导航授权过滤；既有main未用变量/空catch告警不在此片处理。

## Transport取消语义

共享api在请求前与解析响应后检查AbortSignal；预先取消不发请求，完成响应在取消后返回不报告completed。保留上下文revision和原operation恢复语义。34个定向用例通过 `temp/integration-test-runs/20261002T071146Z-63dcfcea/`，tsc通过 `temp/integration-test-runs/20261002T071149Z-bc4e755e/`。客户端取消不代表服务端写入已回滚；未知结果仍须按原operation回查。

## F02.1 作用域Context

新增 `app/WorkspaceScope.tsx`，ScopedWorkspace向子树投影tenant/space/revision，useWorkspaceScope要求显式Provider；不包含文件、凭据或权限判定。测试验证上下文变化后的投影字段及旧页面状态隔离。48个定向用例通过 `temp/integration-test-runs/20261002T071444Z-c6f40c1d/`，tsc通过 `temp/integration-test-runs/20261002T071448Z-db7780cf/`，三文件LSP确认无错误。此Context为后续新壳消费，不替代服务端授权。

## 初始深链请求顺序保护

ScopedWorkspace先在effect中绑定初始URL作用域，再挂载Page子树，避免子页面effect先以旧tenant/space发请求。新增StrictMode初始旧上下文→新managed深链观察测试，所有子effect仅见新scope。49个定向用例通过 `temp/integration-test-runs/20261002T071749Z-1440d375/`，tsc通过 `temp/integration-test-runs/20261002T071753Z-ba23275c/`；两文件LSP确认无错误。首个effect前返回null，不展示旧页面；同scope后续导航不改原行为。

## F02.2 薄bootstrap第一步

原main编排保留于 `web/src/WorkspaceApp.tsx`，同级移动维持全部相对导入和旧CSS顺序；main.tsx只保留主题、分享分支、作用域壳与挂载，仍导出App兼容入口。未删除旧个人壳或业务页面，因此F02.2不能登记完成。tsc通过 `temp/integration-test-runs/20261002T073131Z-f1448dbd/`，47个定向用例通过 `temp/integration-test-runs/20261002T073148Z-5c91885b/`。原main既有空catch告警随文件移动，不属于新增逻辑。

## 旧Agent页面浏览器交互保护

新增legacy-agent-page浏览器测试：真实本地fixture登录、GET /agents显式返回三类旧授权替身；搜索和状态过滤、显示已归档、撤销确认出现及取消均断言；所有Agent非GET拦截并计数，取消前后写请求为零。通过 `temp/integration-test-runs/20261002T131331Z-f7fd74de/`。证明页面交互/确认边界，不证明服务端撤销持久化或新版grant生命周期；不操作真实账号。

## 旧Agent页面迁移

提取features/agents/LegacyAgentPage，筛选/卡片/最近12条活动从WorkspaceApp移出；通过原Agent对象回调撤销和归档，确认及网络请求仍在父层，未迁移或放宽旧权限。保留原受控查询状态、归档过滤、键盘列表；动作按钮明确type=button。首次错误为handleListKey导入路径及测试误把searchbox当textbox，修正后8个定向测试131007Z-2574bab6、tsc131011Z-2ef4e61e、构建131028Z-7b981131通过。旧登录/手机导航/搜索4浏览器例通过 `temp/integration-test-runs/20261002T131107Z-c55f7618/`，不冒称真实Agent撤销全链验收。新版workspace grant页面仍未完成。

## 搜索/文件流程页面编排迁移

新增consumer-flows/FlowPage适配组件，将initialCenter白名单、正式search/旧flows路径、tenant编码和history更新从WorkspaceApp移出；原ConsumerFlows继续承载业务，父层只接原子导航回调。测试覆盖未知center回upload、正式/旧搜索tenant round-trip及保留pathname/search。29个相关用例通过 `temp/integration-test-runs/20261002T125222Z-2d143ebe/`，tsc125228Z-86d1b018，三文件LSP确认无错误。此为一个实际page adapter迁移，其它大页面编排仍未完成，不据此勾F02.2。

## 壳层样式入口收敛

WorkspaceApp在原布局CSS导入位置改用app/workspace-shell.css，后者按原style→finder→workspace-layout→finder-browser顺序引用；未搬到模块顶部，避免改变业务CSS先后关系。生产构建124639Z-9532458f、浅深主题/导航宽度/详情8浏览器例通过 `temp/integration-test-runs/20261002T124733Z-99dc8b87/`。这是过渡入口，不把聚合import冒称旧CSS删除，F02.2仍需实际旧页面编排迁移。

## WorkspaceShell生产根容器

新增app/WorkspaceShell与WorkspaceMain，生产WorkspaceApp根容器改用这两个组件，保持app/mobile-shell/sidebar-collapsed/drive-focused类名和aside/main直接层级。页面数据、权限和弹层仍由现有组件持有；本片不宣称旧页面编排全部拆除。根容器/入口/作用域9测试通过 `temp/integration-test-runs/20261002T124003Z-a59dc1fc/`，tsc124006Z-3a2ce089，三文件LSP确认无错误。随后生产构建通过124221Z-a6a591ea，手机根布局/桌面宽度/详情调宽与刷新/登出六个浏览器用例通过 `temp/integration-test-runs/20261002T124311Z-eef568eb/`。仍保留大chunk警告；详情统计查询的既有替身边界不变，不冒充新业务完整集成。

## 根几何token归位与唯一声明保护

style.css的圆角/控件/导航宽度/顶栏/titlebar环境token移入app/theme/tokens.css，值不变。新增实际遍历src CSS的测试，确保七项核心几何仅在主题源声明并锁定240/68/48。主题与appearance四例123522Z-f3e55eed、UI静态123525Z-524f068d、tsc123527Z-6bbb270a通过。未重复浏览器全量，不宣称未运行的新CSS生产构建通过；F03.1仍需完成整体设计系统验收。

## 语义主题源归位

将shadcn.css中:root/.dark及密度值逐字移动到app/theme/tokens.css，shadcn保留Tailwind映射与控件规则，通过CSS import引用；不修改颜色值。UI静态门123011Z-a4708240、构建123013Z-30715317通过，浅深桌面/手机实际颜色和reduced-motion五浏览器例通过 `temp/integration-test-runs/20261002T123103Z-e6fbc7cb/`。几何token及旧页面局部样式仍需进一步收敛，不据文件迁移登记完整F03.1。

## 登出浏览器闭环

logout-boundary先拦截/auth/logout返回403，断言error可见、主frame无导航、真实/me仍200；移除拦截后执行本地真实登出，登录表单出现、workspace卸载、真实/me变401。构建122549Z-e4393ba3、与旧登录共3浏览器例通过 `temp/integration-test-runs/20261002T122640Z-01d3017a/`。失败响应为注入，成功路径使用可丢弃fixture真实会话；未操作真实账号。

## 登出响应确认与内存清理

新增app/logout.ts的endSession，原api.logout仅在其成功后reload。/auth/logout非2xx返回request_failed、不清身份；网络未知结果保留状态；确认成功才清CSRF与tenant/space并递增revision。15个logout/session/API测试通过 `temp/integration-test-runs/20261002T122226Z-bae2765f/`，tsc122228Z-a6e23851。未删除非秘密隔离偏好，尚待真实浏览器登出回归；客户端清理不替代服务端会话撤销。

## 作用域快照不可变

DataContext字段readonly且每次set后Object.freeze，禁止调用者直接改tenant/space绕开revision和事件；旧快照保持原身份，旧api导出不变。新增Reflect.set拒绝及revision递增回归，与app/上传/API共50例通过 `temp/integration-test-runs/20261002T121732Z-1772abd4/`，tsc121737Z-69d73691。此为客户端一致性保护，不替代服务端权限裁决。

## 同页面深链无副作用回归

ConsumerFlows组件新增upload→search→upload的prop更新测试：tab同步、能力lookup保持四次、不回调用户导航、不发业务POST。10个consumer页面测试通过 `temp/integration-test-runs/20261002T121350Z-525919ab/`，tsc121356Z-dbf8290e。测试替身仅证明客户端副作用边界，不外推后端能力探测正确性。

## 正式搜索入口与旧深链切换

新增search Page/工作台导航，#/search复用ConsumerFlows search中心，不复制搜索数据；手机完整菜单可达。业务tab切换保留tenant并回flows URL。首次浏览器120541Z-cc727480发现旧flows同页面换center未同步initialCenter；ConsumerFlows增加prop变更同步，不重挂容器。9单测120850Z-40a74c51、构建120856Z-c638aa75，以及登录/刷新/旧深链/手机搜索和原consumer-flows五浏览器例通过 `temp/integration-test-runs/20261002T121012Z-6075c965/`。搜索仍沿用原能力探测/诚实降级，不宣称新索引或全部S3搜索已完成。

## 收件深链入口补齐

将现有inbox页登记为Agent组“产物与收件”，修复#/inbox在初始化/hash监听时因navigation缺项回退默认页的问题；手机完整菜单同源可达、底栏任务项保持当前位置。构建115853Z-c4c0a52a、真实本地fixture登录→inbox→刷新→手机菜单返回及触摸/浅深侧栏4例通过 `temp/integration-test-runs/20261002T120008Z-160652eb/`。该页仍是旧Operation收件投影，不冒充workspace submissions审核/归档新UI完成。

## SelectAdapter挂起定位与质量门恢复

V8采样（temp/select-profile/profile.txt）显示jsdom Element.matches递归；nwsapi2.2.27的isModal调用matchesNative(':modal')又回jsdom选择器，且继续查询:fullscreen。脱离React重复20次button.matches(':modal')也在8秒内无法完成（114655Z-3ed6f698），不是交付业务循环。test-setup仅对jsdom不具备原生top-layer的精确:modal/:fullscreen匹配返回false，其余选择器保持真实，生产代码不受影响。移除所有诊断spy/mock后真实SelectAdapter最小复现114922Z-189299e1通过；恢复DeliverCenter SelectAdapter和真实选项交互。

当前全量56文件303例通过 `temp/integration-test-runs/20261002T115255Z-f65ba72a/`，check-ui首次全绿115302Z-a99436b2，tsc115304Z-180a9a10，生产构建115350Z-65150e6e。真实浏览器完整原语浮层、旧业务控制流/详情/手机触摸8例通过 `temp/integration-test-runs/20261002T115440Z-e5ade590/`；浏览器无jsdom适配，既有业务网络替身边界不变。下方未解决记录为历史排查过程，不再作为当前全量状态。

## SelectAdapter独立挂起复现（历史未解决记录）

新增components/SelectAdapter.test.tsx最小真实控件复现：异步选项、空placeholder、受控选择；调试确认值可变为fixture-share、aria-expanded=false、显式卸载也完成，但测试进程随后无法按时退出。已有失败104846Z-d8f9ecd7、105100Z-b4097bce；有界20秒复现113050Z-247ce90d等保留124退出。非零布局、fake timers、MutationObserver桩及浮层局部mock均未解决，已全部撤回，未改生产Select。最终保留不skip的真实回归复现，因此当前Web全量不能称通过；上一轮51/280是历史结果。最新隔离尝试113529Z-7fd59b4c仍超时，不宣称已定位具体底层原因。原交付页仍保留native select，UI静态门未通过。下一步针对最小复现做运行时分析，不盲调超时。

## 搜索模态Checkbox原语

SearchCenter改用既有Checkbox，onCheckedChange保持原modalities数组逻辑。新增选择/取消交互检查发现aria-label与外层label叠加成text text（104458Z-360d3e0c）；移除冗余aria-label而不放宽断言后，42个consumer-flows测试通过 `temp/integration-test-runs/20261002T104545Z-50378dc3/`，tsc104549Z-88b79415。LSP一文件未确认。check-ui仅剩DeliverCenter原生select（104254Z-5d5e9466）；本片尚未运行更新构建后的浏览器模态交互。

## 业务表格原语收敛

UploadCenter批次文件、OrganizeCenter计划项、SearchCenter精确结果改用Table，保留原operation/file/version、行列和aria-label。42个consumer-flows测试103745Z-1ef34bf6、tsc103749Z-91df37d9、构建103851Z-483b933a通过；既有4个consumer-flows浏览器用例通过 `temp/integration-test-runs/20261002T103938Z-ef002457/`。这些浏览器链使用已声明网络替身，不作为真实上传/分享/计划集成验收。全局check-ui剩2项（DeliverCenter select、SearchCenter checkbox），失败证据103826Z-909483fe，选择器挂起原因仍未定位。

## 交付页草稿原语与选择器回退

DeliverCenter两个草稿框改用Textarea，保留原长度限制与请求行为。尝试SelectAdapter导致既有jsdom交互用例挂起（102700Z-3abe4d8f；后续100秒外层中断无完整证据），原因尚未定位；已定向撤回该选择器及两测试文件的交互变更，不提高超时、不跳过断言。仅保留Textarea后9例通过 `temp/integration-test-runs/20261002T103324Z-c0298534/`，tsc103328Z-068237dc。check-ui仍5项失败，证据103407Z-87eeb1ab：一个select、三个table、一个input。没有执行真实分享或扩大分享权限。

## 信息页面Table原语与拖动时序修复

InfoWorkspace五张数据表复用Table原语，保留caption、scope、行列和统计口径；全局check-ui剩7项consumer-flows问题（101606Z-73ec28ef），未放宽规则。首次浏览器101716Z-2134cb60暴露此前拖动断点取消的时序缺口：CSS已隐藏但media change未处理时pointer-up提交440。AppSheet松手同步检查当前media匹配，手机取消不提交；新增确定性不派发media事件的组件回归。36组件例101939Z-db44b3de、构建101946Z-0425a943、详情/reflow4浏览器例 `temp/integration-test-runs/20261002T102034Z-75937581/`通过。此为本目标引入问题修复，不归因表格或测试环境，也未放宽360断言。

## 原语目录依赖检查覆盖

check-ui不再整目录跳过components/ui：所有模块检查原型依赖，仅原语目录豁免native control规则。导出同一checkUI入口，临时fixture回归验证合法原生Button可用、原语导入prototype失败、普通页面原生Button失败、.ts转导出prototype失败。9个检查器测试通过 `temp/integration-test-runs/20261002T101220Z-79d82233/`，tsc101224Z-ca7c9240。全局检查仍12项失败，最新实际CLI证据101005Z-bfe378dd；未将检查器自测通过冒充全局通过。

## 当前详情页原语收敛

FileDetailSheet讨论草稿改用既有Textarea，保留value/maxLength/onChange及本地未发送文案，不新增发送动作。20个详情/宿主测试通过 `temp/integration-test-runs/20261002T100722Z-04391aed/`，tsc100726Z-c268c24d。check-ui剩12项（consumer-flows、InfoWorkspace），最新失败证据100805Z-80e070b5；仅修本片已修改详情页的控件，不放宽检查。

## 生产原型依赖静态门

check-ui复用TypeScript AST检查import/export/dynamic import/require/import type的prototype(s)路径，并覆盖.ts；新production-imports测试实际扫描当前src所有非测试TS/TSX，含原语目录。六种负例、合法内容对照和生产树扫描8例通过 `temp/integration-test-runs/20261002T100433Z-fba9b985/`，tsc100437Z-fd8d51e2。检查的是字面量模块路径，不声称覆盖运行时拼接或构建alias重写。全局check-ui仍因既有13项原生控件失败，最新证据100512Z-96cd99dd，未隐藏或放宽规则。

## 详情组件身份切换隔离

FileDetail.test扩展真实组件rerender链：Alice保存440并调至456→Bob加载独立520→另一实例Alice默认360→无身份调宽不落盘→回Alice恢复456。保留Bob偏好不变并清理fixture键；API使用既有网络替身。15个详情/偏好用例通过 `temp/integration-test-runs/20261002T100030Z-8ec6365d/`，tsc100034Z-5292c8df，新测试文件LSP无错误。这证明组件接线重挂隔离，不冒充两个真实登录账号的浏览器验收。

## 详情偏好刷新回读

现有info-workspace浏览器链按真实/me的instance_id/user.id定位偏好：Enter复位保存360，End保存640，拖至560后Escape恢复640且存储不变；关闭/刷新/重新打开仍640。构建095635Z-6072804d通过，3个详情浏览器例通过 `temp/integration-test-runs/20261002T095722Z-f3362343/`。统计查询仍使用既有替身，此证据仅验收前端偏好生命周期；用户/实例交叉隔离有单测，未冒充跨账号浏览器验收。

## 详情偏好身份接线

SessionView接既有/me.instance_id（旧服务可缺省），WorkspaceApp将instance+user传InfoWorkspace/FileDetailSheet；detail-preference以编码身份生成独立file宽度键，缺实例/用户不持久化，非法值回360，storage故障不阻塞。AppSheet支持initialDetailWidth与显式commit回调，仅键盘/双击/松手提交，不在取消/视口缩小时写偏好；不同身份key重挂sheet。26个相关单测通过 `temp/integration-test-runs/20261002T095349Z-42780569/`，tsc095353Z-b074ddda。尚需真实浏览器刷新回读与取消不覆盖偏好验证；不提前登记F03.3。

## 外部关闭/卸载拖动清理

AppSheet在父级open=false时取消当前拖动、恢复原宽；卸载只清引用并释放pointer capture，不触发额外关闭回调。模拟pointer capture的组件测试验证拖至440后外部关闭/重开回360、卸载释放对应pointerId，19组件例通过 `temp/integration-test-runs/20261002T094733Z-c2a82722/`，tsc094737Z-a32037c7。此为组件生命周期测试，不冒称实机指针丢失场景。

## 拖动期间断点切换

AppSheet监听进入小于768px视口，取消正在进行的拖动并恢复拖前桌面宽度；浏览器验证拖至440→切手机全宽→松手→回桌面360。首次组件测试094142Z-bb150137因jsdom缺matchMedia失败；补既有test-setup的EventTarget媒体查询fixture（不模拟布局）后21组件例094304Z-7d73b1b2、构建094308Z-650e1a0c及详情/手机4浏览器例 `temp/integration-test-runs/20261002T094355Z-da93a204/`通过。LSP两文件未确认，构建包含tsc通过。未保存偏好到本地存储。

## 拖动中断事件验证

复用实际鼠标按下/移动到440px，分别注入pointercancel和lostpointercapture事件，断言回到拖前360px且详情保持打开，随后松开鼠标。定向浏览器用例通过 `temp/integration-test-runs/20261002T093827Z-4aaf46ac/`。中断事件由测试dispatch，不声称物理设备故障复现；本轮仅增强测试，未重构生产实现或重复构建。

## 详情指针调宽与取消

AppSheet显式详情模式使用pointer capture拖动，右侧面板按左移增宽，320–640夹取；松手确认，Escape/pointercancel/丢失捕获回到拖前宽度，Escape拖动期间取消Base UI关闭。浏览器真实鼠标验证360→440→Escape恢复360且dialog仍开，再拖动松手440，Enter复位360；固定version保持。17组件例093525Z-004177b4、构建093530Z-3a7bf4ad，以及详情/手机抽屉4浏览器例通过 `temp/integration-test-runs/20261002T093614Z-61d43a43/`。宽度仅内存，尚未实现实例/用户隔离偏好，也未对pointercancel专项浏览器验证。

## 详情键盘调宽首片

显式detailWidth的AppSheet增加桌面separator，公开320/640/当前值，左箭头增宽16、右箭头减宽16、Shift32、Home/End极值、Enter/双击复位；手机隐藏手柄，普通面板不增加。宽度仅组件内调整，不写跨用户偏好。17个组件/文件详情测试通过 `temp/integration-test-runs/20261002T093127Z-73224ac0/`，tsc093132Z-d03f693a。尚未实现指针拖动与Escape取消拖动，未进行本片真实浏览器键盘验收。

## 文件详情实际视口验证

复用info-workspace浏览器链新增1440/390/320/768/1440切换测量，保留所选version并检查面板无横向溢出。首次失败092524Z-0514b56b揭示Sheet原语sm:max-w-sm把360px限制为336px；详情分支显式覆盖同断点max-width后，构建092705Z-064f395f及全部3个info-workspace用例通过 `temp/integration-test-runs/20261002T092752Z-937a17bd/`。桌面360、手机视口全宽，固定版本与Escape回焦点保留。统计/明细查询存在网络替身，因此不作为完整业务集成证据；LSP一文件未确认，构建类型检查通过。拖拽调宽与隔离偏好仍待实现。

## 文件详情宽度合同首片

AppSheet增加可选detailWidth，限定320–640并裁到100vw，max-md全宽；未传入时保留旧480px面板行为。FileDetailSheet使用360px默认；16个组件/文件详情测试通过 `temp/integration-test-runs/20261002T092200Z-c811e3cf/`，tsc092204Z-a1190e1e。此为宽度合同与组件验证，尚无实际浏览器尺寸验证、拖拽separator或用户偏好保存，不登记F03.3完成。

## 当前壳层集中回归（2026-10-02）

Button原语替换后的当前Web全量51文件280例通过 `temp/integration-test-runs/20261002T091620Z-5c2cabbb/`；生产构建091640Z-3737be4f通过（仍有大chunk警告）；真实本地fixture加限定计数替身的13个浏览器用例通过 `temp/integration-test-runs/20261002T091730Z-26590105/`，覆盖旧登录、桌面/平板/手机导航、主题、触摸、焦点与reflow。不能由这13例外推全部业务e2e。全局check-ui的13项未改业务控件问题仍开放；F02.2尚有旧页面编排/样式隔离，F03详情宿主与正式全导航仍未完整交付，总目标3/11。

## 手机导航组件原语检查

MobileNavigation改用既有Button layout=surface，保留局部几何/主题和禁用语义，不放宽check-ui规则。组件2例通过091430Z-fdbf647a、tsc091433Z-b04bc198。全局check-ui失败证据 `temp/integration-test-runs/20261002T091406Z-bd8951cc/`：13项位于本轮未修改的consumer-flows/info-workspace原生控件，未顺手改写或删除断言；不称全局UI质量门通过。新Button实际触摸/主题回归待下一稳定构建。

## 手机完整导航分组

手机AppSheet复用navGroups的权限过滤与项目归属，显示工作台/Agent/管理三个具名section；不继承桌面折叠状态，所有允许入口可达。构建091016Z-3d2fe4bd；触摸验证分组heading与Agent所属region，焦点循环/断点恢复/小视口滚动及reduced-motion三个浏览器用例通过 `temp/integration-test-runs/20261002T091105Z-d127ee40/`。仍只包含当前已有页面，不声称全部正式业务导航齐备。

## 三组导航对齐

现有可用导航归入工作台/Agent/管理；Agent不再混入旧系统组。旧system偏好键保留并用于管理，新agents组默认展开，旧collab值不迁移以免错误隐藏新Agent；团队/分享/治理入口保留。不为未实现知识资源挂空链接；inbox尚未在顶层navigation登记，不冒称收件页新导航已交付。5单测090641Z-fd8ec9ed、构建090643Z-8e5ac58f通过；旧工作台完整回归/浅深对比度/计数/触摸5浏览器例通过 `temp/integration-test-runs/20261002T090732Z-da48b888/`。正式全部导航条目仍需后续业务页接入。

## 导航URL生成接入

workspacePageURL生成home/files/tasks/storage正式路径并编码tenant；WorkspaceApp普通导航与返回云盘列表已消费该函数，旧治理/成员/统计特殊别名保留。路由/作用域22例通过 `temp/integration-test-runs/20261002T085908Z-eaf44815/`，tsc085912Z-90871990。新增正式路径不意味着对应新业务聚合实现，随后构建090117Z-02cf3a84通过；生产触摸五栏逐项断言settings/agents/tasks/files/home URL，与旧登录/桌面宽度4个浏览器用例通过 `temp/integration-test-runs/20261002T090207Z-817e3ea9/`。

## Canonical路由兼容首片

补home→overview、tasks→transfers、storage→connections的类型化映射，保留旧路由和tenant值；现有页面与权限仍由原应用处理。路由/作用域21例通过 `temp/integration-test-runs/20261002T085512Z-13353037/`，tsc085516Z-3692b65b。这里只让正式URL可达既有功能，不宣称新首页聚合/任务调用/存储能力全部实现；尚未接知识资源等未实现页面，未新增虚假数据。

## 浏览器触摸输入验证

新增mobile-touch.spec.ts，以isMobile/hasTouch/deviceScaleFactor=2的浏览器上下文实际tap五栏全部入口及抽屉设置/关闭按钮，检查aria-current、无横向溢出和选择后关闭。通过 `temp/integration-test-runs/20261002T085203Z-2f567cb9/`。这是Chromium模拟触摸事件，不是物理手机实机验收；未改源码，不重复构建。

## 抽屉实际减少动效验证

既有shadcn.css已覆盖OS reduced-motion，不新增CSS。shell-reflow扩展计算样式检查sheet-content及sheet-overlay各transition/animation duration≤0.00001s、scroll-behavior=auto，保留布局/焦点断言；通过 `temp/integration-test-runs/20261002T084951Z-e26fb562/`。只验证当前导航抽屉与遮罩，不外推所有业务动画。

## 缩小可用视口与减少动效偏好

shell-reflow.spec.ts使用720×450（1440×900在200%缩放时的可用CSS布局空间）及prefers-reduced-motion=reduce验证底栏可见、无水平溢出、设置可达、抽屉内退出按钮滚动可达、Escape恢复焦点。证据 `temp/integration-test-runs/20261002T084716Z-03d31b0e/`。这是等效布局空间测试，不是真实浏览器200%缩放或所有动画时长验收；本轮未改生产代码、不重复构建。

## 顶栏基础高度单源

新增workspace-topbar-height=48px，移除旧finder 65/60px与重复手机48px覆盖，将style/workspace-layout基础高度改为同一token；高度auto保留业务操作换行能力。真实浏览器测量桌面基本顶栏48px，手机壳/浅深侧栏回归4例通过 `temp/integration-test-runs/20261002T084508Z-9b51918e/`，构建084426Z-f67b24fa。复杂治理工具条可自然增高，不声称所有页面始终固定48px。

## 折叠计数前端浏览器验证

navigation-count.spec.ts用网络替身提供一项verifying及一项completed，折叠组只显示1；触发现有刷新事件后两项均completed，计数消失且组仍折叠。构建083814Z-4c8eb01d；计数与宽度浏览器两例通过 `temp/integration-test-runs/20261002T083859Z-71af9b9f/`。仅前端投影验证，不是Operation真实发布或持久化验收；LSP一文件未确认。计数文字对比度尚未专项测量。

## 折叠分组已加载任务提示

折叠工作台组按现有ops投影展示未结束任务数量，按operation ID去重；completed/cancelled排除，未知状态不冒充完成。说明明确“当前已加载列表”，不当服务端总计，不新增任务状态。nav-groups四例通过 `temp/integration-test-runs/20261002T083533Z-b4365ea0/`，tsc通过083535Z-54833943。尚未完成非空任务下的浏览器计数/对比度验证，不据此登记F03.2完成。

## 折叠分组当前位置提示

分组折叠时在标题内显示当前页名称，保留原按钮accessible name和展开行为；重新展开后aria-current仍指向原页。浏览器真实点击工作台折叠/展开验证通过，与浅深侧栏3例证据 `temp/integration-test-runs/20261002T083109Z-315e7a9c/`；构建083022Z-7ace8da7。尚未补齐折叠组任务/收件数量投影，不能称分组全部状态完成。

## Precision Pine侧栏与导航行

shadcn.css侧栏语义token采用冻结canvas/selected色；style.css在sidebar局部映射文字/hover/边界，不让正文或Finder目录继承反色。导航行44px；首次浏览器发现旧finder尾部38px覆盖（082332Z-7e330c9c），删除该覆盖与workspace-layout重复40px后通过。构建082527Z-44823df0；浅深侧栏实际渲染文字对比度≥4.5、行高≥44以及手机主题/桌面宽度5例通过 `temp/integration-test-runs/20261002T082615Z-3942a1cc/`。已查看sidebar-light.png，侧栏深松绿、正文浅色；不声称全站或所有hover状态对比度完成。

## 退役手机侧栏CSS清理

从finder.css移除旧62px固定sidebar/主区左偏移/隐藏toggle规则；从style.css移除旧横向67px手机sidebar与按钮编排；workspace-layout.css删除重复手机偏移与旧分组规则。保留文件/阅读器工具条响应式规则。构建081740Z-64f98f27；手机抽屉、浅深主题、快捷键、桌面平板宽度5个浏览器用例通过 `temp/integration-test-runs/20261002T081823Z-279159d4/`。此为已不挂载DOM的旧规则删除，不声称全部旧壳/CSS均退役。

## 导航快捷键避让

Ctrl/Cmd+B复用isTypingTarget避让contenteditable及表单，另避让文字选择、已消费事件、输入法组合与Alt组合；手机只切换抽屉，不写桌面偏好。真实浏览器验证富文本/选中文字不收起、不写偏好，普通快捷键收起，手机快捷键打开抽屉并保持原偏好。构建081316Z-fdf85a71，导航/宽度/手机回归3例通过 `temp/integration-test-runs/20261002T081402Z-ff352945/`。未对所有阅读器快捷键作全覆盖声明。

## 默认导航值不持久化

修复原effect将响应式默认值直接存为用户偏好的问题：仅按钮/键盘触发展开收起后写storage，初始tablet rail及desktop expanded不写。浏览器验证无偏好1024刷新68、1440刷新240、storage始终null；手动展开后存false并刷新保留。构建080919Z-6edd4e42，手机/桌面回归两例通过 `temp/integration-test-runs/20261002T081009Z-47f7cf66/`。不自动清除历史用户偏好。

## 手机主题绑定修复

MobileNavigation原生CSS改为使用实际定义的ui-background/ui-foreground/ui-border/ui-accent/ui-ring，修复不存在的变量导致透明背景/颜色回退。构建通过080506Z-fac4df65；浅深色computed style与五个按钮至少44×44px检查、手机抽屉回归3例通过 `temp/integration-test-runs/20261002T080555Z-67903bcc/`。已查看mobile-dark.png，主题与选中项可见；不将颜色解析/尺寸测试当作完整对比度、实机触控或200%缩放验收。

## 平板导航默认偏好

initialSidebarCollapsed在无有效偏好时以768–1099px为默认rail；显式true/false优先，storage不可用回退视口，不因resize强制改用户选择。4个单测通过075908Z-c1873f63，tsc075910Z-1cfc3bf2，构建080030Z-886b8261。浏览器1024初始68px、手动展开后刷新240px，与手机/桌面回归两例通过 `temp/integration-test-runs/20261002T080113Z-415bad43/`。尚不等于所有平板详情布局/主题验收。

## 桌面导航宽度单源

style.css定义navigation-expanded-width=240px、navigation-rail-width=68px；移除finder.css和workspace-layout.css的重复224/64声明及style.css重复断点值，既有sidebar-width消费语义token。构建通过075450Z-d924a375；真实浏览器测量展开240、收起68、再次展开240并检查无横向溢出，与手机壳回归两例通过 `temp/integration-test-runs/20261002T075605Z-daf42ca3/`。此为桌面几何切片，不代表完整Precision Pine主题或平板默认rail完成。

## 手机抽屉焦点与断点修复

扩展浏览器测试：20次Tab后焦点仍在dialog内、Escape关闭返回顶栏按钮、打开抽屉后切桌面再回手机保持关闭。初次瞬时焦点断言失败074753Z-4b4d16ac，改为等待Base UI焦点转移后通过该段；随后真实复现断点状态残留074859Z-2fff2bab。WorkspaceApp现在离开mobile断点清空抽屉状态，构建075007Z-6bca0c9a，浏览器完整通过 `temp/integration-test-runs/20261002T075049Z-0624eb4b/`。未修改共用AppSheet焦点实现，旧main静态告警不在此片处理。

## 手机生产壳接线

小于768px时WorkspaceApp不再挂旧sidebar DOM，底部MobileNavigation复用原页面/allowedNav；顶栏按钮打开现有AppSheet完整菜单，保留设置/治理/退出入口。mobile-shell局部几何规则隔离旧桌面CSS，后续仍需清理旧叠加规则，不声称F02.2全部完成。tsc通过074247Z-21d4ffda，构建通过074304Z-64e97054；真实本地浏览器手机壳+旧登录3例通过 `temp/integration-test-runs/20261002T074427Z-a1d5e980/`。验证320/390/767底栏在底部无页面横向溢出、菜单Escape关闭、1440恢复桌面；已查看mobile-shell.png，五栏可见，旧空网盘内容仍保留。LSP两文件未确认，不称全clean。抽屉焦点回归、触摸/主题/缩放及桌面新壳尚待验收。

## 手机五栏组件初片

新增MobileNavigation及局部CSS module：固定首页/文件/Agent/任务/我的，使用既有Page标识和外部onNavigate，allowed由shell提供，不复制路由状态或授予权限。按钮56px高并带safe-area、键盘focus，使用既有语义颜色。四个组件/reducer用例通过 `temp/integration-test-runs/20261002T073756Z-166bc123/`，tsc通过 `temp/integration-test-runs/20261002T073759Z-0d3c37a5/`。尚未挂生产壳，未截图/触控/对比度验收，不能称手机五栏已交付。

## Bootstrap分支保护

新增bootstrap.test.tsx，通过mock挂载器验证旧分享分支与ScopedWorkspace互斥、主题初始化一次、无root不启动且兼容default export。三文件21个测试通过 `temp/integration-test-runs/20261002T073422Z-45fd88c5/`，tsc通过 `temp/integration-test-runs/20261002T073426Z-9ac6afd5/`。这是入口接线单测，不冒称分享业务或新视觉通过；F02.2仍待旧壳替换。

## AuthGate页面编排提取

新增 `app/AuthGate.tsx`，完整loading/匿名登录DOM移出main，保留旧类名、OIDC链接、demo入口、KeyLoginForm和governance账号恢复条件。main仍负责会话状态与业务reload，不把UI门当服务端授权。新增loading、能力发现未返回、key委托、OIDC/demo条件测试。53个定向用例通过 `temp/integration-test-runs/20261002T072540Z-adb027ce/`，tsc通过 `temp/integration-test-runs/20261002T072544Z-42a9267e/`。LSP两文件未确认，不称全clean；登记F02.1前还需提取后稳定浏览器回归。

## 提取后集中验证与下一步

当前Web全量47文件261用例通过 `temp/integration-test-runs/20261002T071928Z-3f8bcebb/`；生产构建通过 `temp/integration-test-runs/20261002T071954Z-b374ba19/`，保留大chunk警告。真实本地fixture浏览器4例通过 `temp/integration-test-runs/20261002T072037Z-85910bbe/`，覆盖两种登录身份、管理员入口隔离、登录提示、旧深链和桌面手机导航。这些是现有壳层回归，不是新视觉验收。

F01.1/F01.2已通过任务CLI登记完成，前文未登记描述是历史进度。AuthGate提取后构建通过 `temp/integration-test-runs/20261002T072717Z-e5626158/`；登录/权限/双端旧深链浏览器4例通过 `temp/integration-test-runs/20261002T072805Z-40caf720/`。F02.1已通过任务CLI登记完成：保护模块已从旧编排中提取，原接口与恢复身份保留。下一步进入F02.2薄bootstrap/旧壳替换，不再重复拆分基础模块；该任务不授权删除仍被业务页面使用的阅读器、治理或管理入口。新壳及CSS切换前读取冻结UI Spec和本地设计工作流。
