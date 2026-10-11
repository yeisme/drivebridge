## ADDED Requirements

### Requirement: 独立 Agent 最小授权
客户端 MUST 展示每个 Agent 的协议、目录、动作、有效期、授权状态和真实调用来源，且普通网盘不依赖 Agent。

#### Scenario: 首次接入
- **WHEN** 在零 Agent 的个人空间打开授权表单
- **THEN** 默认只选读取与长期（expiry_mode=never、expires_at=null），建议/收件分别选择；可选 duration 的 1/24 小时及 7/30/90/365 天，保存不自动探测或操作。

#### Scenario: 旧授权消费
- **WHEN** 服务器返回旧 read/write scope Agent
- **THEN** 标记旧版授权，不伪装为新窄权限，不自动重签或撤销。

### Requirement: 授权期限与生命周期交互
客户端 MUST 使用服务端期限和 revision，提供禁用、撤销、轮转与期限修改，不能使用本机时钟重置期限。

#### Scenario: 期限修改和冲突
- **WHEN** 人类将未过期 grant 切为长期或任一限时且另一设备已修改
- **THEN** 携旧 revision 的请求冲突，保留表单重读；成功期限从服务端修改成功时刻算，验证连接/轮转不续期。

#### Scenario: 轮转及终态
- **WHEN** 人类轮转或试图恢复过期/撤销 grant
- **THEN** 轮转保持原 grant 身份和期限、旧凭据立即失效；丢响应先 lookup 原轮转键，终态只能显式新 grant 接入，不能静默复活。

### Requirement: Agent 卡片与服务端组织
客户端 MUST 提供新版卡片、分组 CRUD、归组与置顶并消费服务端个人组织数据，跨设备一致，不用浏览器存储作为真源。

#### Scenario: 组织刷新与冲突
- **WHEN** 在设备 A 归组/置顶后刷新或设备 B 读取、并以旧 revision 修改
- **THEN** 展示服务器事实，冲突保留意图并重新读取；本人之外不可见，不改变文件授权。

#### Scenario: 删除分组与焦点
- **WHEN** 删除当前组或在手机/键盘操作归组与置顶
- **THEN** 删除仅解除归属、保留置顶和 Agent 授权；触控和焦点按 UI Spec 恢复，不因重排丢失焦点。

#### Scenario: 展开与筛选偏好
- **WHEN** 用户操作全部展开/收起或授权/任务状态筛选并刷新
- **THEN** 仅按instance/tenant/user/space保存非秘密本机偏好并校验enum；不修改服务器分组/置顶或授权，搜索/调用对象/日期仍为会话状态。

### Requirement: S2标签建议审阅
客户端 MUST 在固定版本文件详情提供按真实Agent来源的标签建议与人类采纳/拒绝，不等待完整信息工作台。

#### Scenario: 建议采纳冲突
- **WHEN** 人类选择建议子集提交而文件版本或建议revision已变化
- **THEN** 保留选择并重读，禁止强制覆盖已有人工标签；未知结果回原建议和幂等请求，不新建决定；Agent不可自批。

### Requirement: 有界观察与原请求恢复
客户端 MUST 使用安全读取轮询而不自动重试未知写入，不把网络状态当业务状态。

#### Scenario: 后台与退避
- **WHEN** 当前页存在活动任务、进入后台或收到429
- **THEN** 前台活动约2秒/空闲摘要约30秒、后台暂停、无重叠请求；失败按2/4/8/16/30秒带抖动退避并尊重更长Retry-After，401/403停止重试，回前台重鉴权。

#### Scenario: 写入丢响应
- **WHEN** 授权管理、建议决定或归档请求结果未知
- **THEN** 不乐观显示成功、不换键重建，先原键或原资源查询；切换身份后迟到响应不得更新新上下文。

### Requirement: 凭据与调用记录
客户端 MUST 对一次 secret 进行非持久化交付，并区分授权、连接观察和单次调用结果。

#### Scenario: secret 响应未知
- **WHEN** 创建请求超时但 server 已生成 grant
- **THEN** lookup 原键并展示 metadata；需要时人类显式重新签发，不重复创建 Agent。

#### Scenario: 有调用失败但授权有效
- **WHEN** 用户按日期和 Agent 筛选调用
- **THEN** 显示失败事实和原操作入口，不把 grant 自动标为已撤销；筛选在服务器执行。

#### Scenario: 外部接入而非演示执行
- **WHEN** 用户配置 MCP/API、显式验证或查看任务进度
- **THEN** 配置使用占位凭据与可发现工具 schema；验证不派发任务/不证明回存成功，正式 UI 无执行演示调用；真实调用才有记录，进度来自持久 Operation/Plan 原身份，刷新不消失。

### Requirement: 计划确认与原任务恢复
客户端 MUST 使用服务端 revision、impact digest、逐项结果和原 owner ID，不自造可执行任务状态。

#### Scenario: 修改上下文后确认
- **WHEN** 选择、源版本或策略在预览后改变
- **THEN** 旧确认不可使用，显示 stale 并重新取得完整预览。

#### Scenario: 部分成功和迟到结果
- **WHEN** 同一任务部分完成或响应晚于上下文切换
- **THEN** 成功项不重复，结果保留原会话/任务归属，取消不宣称已回滚修改。

### Requirement: 文件收件与人工归档
客户端 MUST 分开表达已上传、待文件验收、已接收/拒绝和归档，并保留真实 Agent/原版本。

#### Scenario: 接收产物
- **WHEN** 人类预览后接受 Agent 的固定版本
- **THEN** 只记录文件接收；归档和领域采纳各有独立结果，不自动执行资源。

#### Scenario: 归档冲突或重复点击
- **WHEN** 目标同名或同一个归档请求被重复发送
- **THEN** 显示冲突供选择或回原 operation，无静默覆盖、重复文件。

#### Scenario: 可读收件与旧响应兼容
- **WHEN** 人类或原作者读取已授权收件，原上传operation与space/grant/Agent/owner/purpose全部匹配
- **THEN** 可选展示字段从原operation及当前可读Agent名称投影，文件head更新不改原文件名/type/size；UI优先显示文件名、来源、状态，技术ID可展开，缺字段保持未知，caller自述不得作为认证名。

#### Scenario: 非文本原版本检查和下载
- **WHEN** 人类打开PNG/PDF或不支持内嵌的二进制提交
- **THEN** 重查submission与受权固定版本后使用安全阅读器或显式原字节下载；不信任Agent提供的URL，不自动转换或执行HTML，下载前重新鉴权，context/失权/卸载后迟响应不得启动检查或下载。

#### Scenario: head漂移与读取失败
- **WHEN** 文件head在提交后更新，或文本读取失败后用户下载原版
- **THEN** 检查仍保持原提交版本，过时人工决定由服务端拒绝；下载状态不得把读取失败改显示为空文件，原版本内容/权限不能被当前head替换。

### Requirement: 保留助手与跨端边界
客户端 MUST 保留内置助手和原生能力的后续需求，未就绪时不以 mock 结果冒充运行。

#### Scenario: 内置助手未接入
- **WHEN** 从文件上下文请求助手
- **THEN** 展示未就绪原因和外部 Agent 固定引用路径，不自动调用模型。

#### Scenario: 跨端继续
- **WHEN** 手机打开任务深链
- **THEN** 重新鉴权并读取同一个 owner ID/ref，深链不携带永久凭据，也不扩大设备权限。
