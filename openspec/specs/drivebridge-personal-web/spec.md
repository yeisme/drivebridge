# drivebridge-personal-web Specification

## Purpose
定义个人文件网页对现有身份、存储、传输及 Agent 授权服务的可视化消费合同，覆盖持久传输、产物归档、登录 Key、统一存储与回收站。资源归属和写入权限继续由服务端校验。
## Requirements
### Requirement: Personal identity isolation
系统 SHALL 校验用户、Agent 和资源归属，OIDC 登录不自动授予 OneDrive 文件访问。

#### Scenario: Another user requests a file
- **WHEN** 用户请求他人的文件、版本、操作或连接
- **THEN** 返回拒绝且不泄露内容或存在性

#### Scenario: Agent authority revoked
- **WHEN** Agent token 已撤销、过期或超出目录/操作范围
- **THEN** 拒绝后续请求

### Requirement: Durable file transfer
系统 SHALL 保留稳定文件身份、不可变版本和原操作恢复。

#### Scenario: Interrupted upload
- **WHEN** 分片上传中断或服务重启
- **THEN** 查询原 operation 及已接收分片，校验相同源文件后继续，不创建重复文件

#### Scenario: Replayed complete
- **WHEN** 客户端重放已完成操作
- **THEN** 返回同一文件版本与回执

### Requirement: Safe user and Agent interfaces
系统 SHALL 通过统一服务提供网页/API/CLI/MCP，临时凭据不得进入普通 MCP 结果和日志。

#### Scenario: Remote client path
- **WHEN** 远程 MCP 收到客户端文件路径
- **THEN** 不把路径当作远端文件，指引宿主通过 HTTP 传输字节

#### Scenario: User changes file version
- **WHEN** 当前版本与调用者指定版本不同
- **THEN** 不覆盖文件并返回版本冲突

### Requirement: Artifact inbox page
网页 SHALL 提供产物收件箱页：只读展示近 30 天操作结果（完成与失败），轮询刷新，失败项按错误码给出用户可理解的文案并可跳转传输中心恢复，不提供任何生成入口。

#### Scenario: Completed artifact
- **WHEN** 收件箱中的操作已完成
- **THEN** 展示产物名、来源与大小，可进入版本选择流查看固定版本

#### Scenario: Failed artifact
- **WHEN** 操作失败或中断
- **THEN** 按错误码显示中文文案，提供到传输中心的恢复入口，不在收件箱内重试传输

### Requirement: Managed archive form
版本面板 SHALL 支持把选中固定版本归档到目标空间与路径：提交走 `/archive`，路径冲突由服务端自动改名，用户不做覆盖确认；`invalid_archive` 等错误码有明确文案。

#### Scenario: Archive from pinned version
- **WHEN** 用户在版本面板选择目标空间与路径提交归档
- **THEN** 提交归档请求并以操作结果反馈，绝不覆盖既有文件的承诺由服务端保证

### Requirement: Dual-path login with key exchange
登录页 SHALL 按 `/auth/mode` 如实呈现登录路径：OIDC 可用时提供账号登录，密钥登录作为输入路径始终可用；密钥换发会话后浏览器不保留密钥。

#### Scenario: Key-only deployment
- **WHEN** 部署未配置 OIDC 且用户在登录页
- **THEN** 仅呈现密钥登录输入，无 OIDC 按钮

#### Scenario: Wrong key
- **WHEN** 提交无效密钥
- **THEN** 显示登录失败提示，不区分不存在与已吊销

#### Scenario: No key persisted client-side
- **WHEN** 密钥登录成功后检查浏览器存储
- **THEN** 密钥不出现在 localStorage、sessionStorage 或 URL

### Requirement: Admin key-user management page
用户 SHALL 在网页端查看本人 Key 元数据、复制真实 Key、再次复制、遮罩查看与确认轮转。admin 的既有批量签发及吊销按服务端权限提供，管理身份 SHALL NOT 获得他人 Key 的取回能力。输入框和下拉框 SHALL 具有一致的绿色主题、聚焦与禁用样式，并支持键盘与窄屏。

#### Scenario: Admin issues users
- **WHEN** admin 提交批量签发表单
- **THEN** 明文密钥在签发弹窗展示并可复制，列表即时更新；关闭后仅所属用户可从本人 Key 页面取回

#### Scenario: One-time display
- **WHEN** 签发弹窗关闭后再次进入管理页
- **THEN** 列表只有生命周期元数据，无任何明文或哈希

#### Scenario: Revoke with confirmation
- **WHEN** admin 吊销某密钥并确认
- **THEN** 该行翻转为已吊销状态

#### Scenario: Hidden from non-admin
- **WHEN** 普通用户登录
- **THEN** 用户可进入本人 Key 页面，不出现管理员签发和跨用户管理动作

#### Scenario: Repeat copy and rotation
- **WHEN** 用户复制本人有效 Key 或确认轮转
- **THEN** 复制内容为实际密钥，支持再次复制；轮转显示旧 Key 立即失效的影响，成功后可查看和复制新 Key

#### Scenario: Clipboard unavailable
- **WHEN** 浏览器拒绝自动复制
- **THEN** 显示默认遮罩的可手动选择密钥面板；关闭面板后清除明文状态

### Requirement: 统一存储可视化

网页与本机只读视图 SHALL 按后端身份呈现空间与连接，能力标签不得超出既有 `backend_capabilities` 矩阵，未识别的存储类型 SHALL 诚实降级而不是冒充已知类型。

#### Scenario: 混合后端空间徽标
- **WHEN** 本机视图的空间投影携带 `backend`（如 `onedrive` 与 `s3`）
- **THEN** 每个空间显示后端标签与对应能力（OneDrive=传输·临时直链·参考交接；generic=仅传输），服务端 `/spaces` 未携带 `backend` 时不猜测、保持静默

#### Scenario: 未识别存储类型
- **WHEN** 连接或空间出现客户端未收录的 kind
- **THEN** 显示「未识别的存储类型/后端」并按既有授权呈现，不静默归为 S3 或 OneDrive，不阻断浏览

### Requirement: 本机只读存储浏览

本机只读视图的存储浏览 SHALL 区分实时目录与已观察索引两种数据源并如实标注，浏览不得提供任何写入口。

#### Scenario: owner 运行时的实时目录
- **WHEN** owner 以 `--enabled --insights` 运行且用户浏览某空间目录
- **THEN** 显示实时列表并标注「列表会更新已观察索引」；超限截断时提示 partial

#### Scenario: owner 缺席的诚实降级
- **WHEN** owner 未运行、未启用文件操作或列表失败
- **THEN** 降级为已观察索引并显示对应原因与「非全盘」声明，不伪造实时结果

#### Scenario: 未观察目录不出现
- **WHEN** 远端存在从未被观察过的子目录
- **THEN** 目录导航不包含该目录；用户可通过手动路径输入访问已知路径，输入 `..` 等逃逸段被拒绝

### Requirement: 跨平台可安装

网页 SHALL 以同一份资产服务桌面与移动浏览器，并提供可安装的应用清单；不得为跨平台引入独立客户端壳。清单可声明 `window-controls-overlay` 作为 `standalone` 的优先显示模式，以及指向现有页面的快捷方式。

#### Scenario: 手机安装为应用

- **WHEN** 用户在手机浏览器打开已部署的网页并选择「安装/添加到主屏」
- **THEN** manifest 提供名称、主题色与图标，应用以独立窗口打开；不新增运行时、更新通道或原生包

#### Scenario: 本机视图不参与安装

- **WHEN** 用户打开 `drivebridge view` 的本机只读视图
- **THEN** 该一次性启动页面不声明安装清单，保持 loopback 与一次性 fragment 语义

#### Scenario: 平台状态如实呈现

- **WHEN** 平台能力未经验证（如 Windows 实机）
- **THEN** 分发与文档按「未实机验收的候选」标注，不宣称可用

#### Scenario: 桌面 overlay 回退 standalone

- **WHEN** 浏览器不支持 window controls overlay
- **THEN** 应用仍以 standalone 窗口打开，功能页可用

### Requirement: Key 管理支持删除
网页 SHALL 在 Key 行展示吊销与删除（当服务端返回 can_revoke/can_delete）。删除须确认。最后一把有效 Key SHALL NOT 出现删除按钮。

#### Scenario: 删除备用 Key
- **WHEN** 用户新建第二把 Key 并确认删除
- **THEN** 该行变为已归档/已吊销，不能再复制明文

### Requirement: Key 管理支持禁用和启用
网页 SHALL 在 can_disable 时提供禁用确认，在 can_enable 时提供启用。禁用中的 Key 显示「已禁用」，且不能复制明文。

#### Scenario: 禁用后再启用
- **WHEN** 用户禁用一把备用 Key 再启用
- **THEN** 状态从已禁用回到有效

### Requirement: 回收站批量与定时清理
回收站 SHALL 支持全选、恢复所选、永久删除所选，并展示空间自动清理天数。

#### Scenario: 批量恢复
- **WHEN** 用户全选回收站文件并恢复所选
- **THEN** 这些文件回到网盘列表

### Requirement: 楔子页面共用列表手感
传输、收件箱、Agent、分享列表 SHALL 可键盘移动焦点。空状态 SHALL 提供一个下一步按钮。取消传输 SHALL 先确认。

#### Scenario: 传输空状态
- **WHEN** 没有传输记录
- **THEN** 显示「去网盘上传」

#### Scenario: Agent 筛选
- **WHEN** 用户选择「有效」
- **THEN** 只显示未撤销且未到期的 Agent

