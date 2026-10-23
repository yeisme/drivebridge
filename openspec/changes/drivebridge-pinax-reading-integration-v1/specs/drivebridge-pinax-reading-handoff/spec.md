## ADDED Requirements

### Requirement: 单客户端与领域 owner 分离
系统 SHALL 在 `cli/drivebridge` 内建设共享 Web/Electron 阅读体验，通过公开合同消费 `cli/pinax`；不得新增 Pinax Reader、笔记后端或恢复 Workbench 作为依赖。

#### Scenario: 用户打开已绑定笔记
- **WHEN** 用户在 DriveBridge 知识库打开 Pinax 笔记
- **THEN** 笔记身份、revision、正文和关系来自 Pinax，客户端不访问私有存储或复制 canonical 图谱

### Requirement: 人类阅读不扩大 Agent 默认权
双方 SHALL 将显式授权的人类完整阅读与既有 bounded Agent 投影分开登记、生成及验收，能力只有真实 binding 就绪后才可标记可用。

#### Scenario: 只有旧 MCP render 可用
- **WHEN** DriveBridge 发现 Pinax 缺少 reader binding 或正文 grant
- **THEN** 显示不支持或未授权，不调用普通文件读取补齐，不改变旧 MCP 正文开关

### Requirement: 显式绑定建立跨入口保护
DriveBridge SHALL 在显式绑定前验证可识别的管理范围、重叠连接与在途写入，并在所有应用可控制写入口执行同一保护；旧挂载不自动迁移。

#### Scenario: 文件列表尝试绕过笔记权限
- **WHEN** 已绑定 Pinax 范围通过普通文件入口收到覆盖、移动或同步写请求
- **THEN** DriveBridge 拒绝并指向原 owner，Pinax 不可用也不允许直接写文件

### Requirement: 三平台与分层验收不能互相替代
桌面交付 SHALL 分别保留 macOS、Windows、Linux 实机证据，工程、签名发布及自然使用结论分开；现有 Pinax 真实验收保持原分母与窗口。

#### Scenario: 仅 Linux fixture 和交叉编译通过
- **WHEN** macOS 或 Windows 尚无实机安装、启动和离线阅读证据
- **THEN** 对应任务保持未完成，不宣称三平台桌面已交付，不勾选旧 Pinax 真实验收

### Requirement: 保留后续能力并复用原任务
规格 SHALL 保留编辑、捕获、提案审阅、整理及 DriveBridge 移动客户端要求，复用原 F/B 任务而不重建平行任务系统。

#### Scenario: 阅读首片完成
- **WHEN** 首片只读链路通过
- **THEN** 后续写入和移动端仍有明确交接任务，不因首片排除被删除或宣称已实现
