## ADDED Requirements

### Requirement: 显式绑定与完整预检
应用服务 SHALL 在人类确认正文/关系/附件scope和保护影响后原子登记binding和guard；预检只授metadata并检查canonical root、content root、已登记可写连接/别名及在途操作。取消或无法证明覆盖时不得持久绑定/读取正文；确认后新建会话，不原地提升预检权限。

#### Scenario: 旧挂载有未知可写别名
- **WHEN** 预检不能证明旧overlay/FUSE或另一state写入口受同一guard保护
- **THEN** 返回managed_scope_unverifiable，保留原数据和挂载，不自动迁移或init

### Requirement: 全应用入口不能绕过owner
受管范围 SHALL 由CLI/MCP/HTTP/UI/worker的共享应用裁决保护，覆盖正文、属性、标签、覆盖上传、移动删除、祖先目录、同步和发布；新连接及发布前也复检。

#### Scenario: 通过文件保存或后台同步修改笔记
- **WHEN** 请求的源/目标/祖先影响已绑定范围
- **THEN** 拒绝managed_owner_required，不因Pinax离线或普通write权限降级为直接文件修改

### Requirement: 显示移除与释放保护分离
隐藏来源 SHALL 保留guard，释放保护必须单独确认、CAS并排除未决mutation；旧挂载writable事实不能替代新保护许可。

#### Scenario: 删除知识库导航来源
- **WHEN** 用户只隐藏来源
- **THEN** vault与阅读状态不删除，CLI和worker继续受guard限制

### Requirement: 降级保持保护
新保护state SHALL 声明最低兼容writer版本，并证明保护域内旧writer在写前可由已有admission机制拒绝；无法证明时不得绑定，不能仅新增字段便声称旧程序已受控。关闭UI不清保护，外部编辑器/未登记外部程序不在保护保证范围。

#### Scenario: 回退到不理解guard的版本
- **WHEN** 旧binary打开带active guard的state
- **THEN** 阻断写入并提示使用兼容owner，不恢复旧数据库或删除登记来继续
