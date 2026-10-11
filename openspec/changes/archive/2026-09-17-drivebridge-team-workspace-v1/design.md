# 团队服务实现

## 归属与兼容

团队、邀请、存储分配和权限状态归本服务；客户端只呈现服务端结果。Team 增加 description、archived、revision、updated_at，新增 TeamInvitation 和 TeamStorage。所有持久化使用 GORM。旧团队默认 active，旧平台团队空间补齐平台分配；旧 teams、team-member、spaces、permission 请求继续受理，其中 spaces 保留直接空间对象响应。旧请求不强制版本号，新 team-* 动作强制 expected_revision；所有入口共用新规则。

最后团队管理员和最后空间数据负责人保护是计划明确批准的安全收紧。旧团队成员／空间权限及租户移除入口不可绕过。保护不为客户端增加管理员权限，不把 team 管理身份当作编辑授权。不删除旧 API，消费者由本轮 Web 同步更新。

## 事务与存储

沿用单 owner governanceMu 序列化治理动作；事务内权限查询使用同一个 GORM 事务连接，避免单连接 SQLite 自锁。版本检查、成员变化、邀请变化、团队 revision 与审计／站内通知在同一事务提交。

TeamStorage 保存连接 ID、团队、租户和分配人。连接密钥仍在原 Connection owner。候选仅平台连接或当前租户当前管理员本人的连接；绑定需要 S3 且未停用。团队网盘使用 storageForSpace 按具体 Space owner/id/connection 验证分配，再解封原 owner 凭据。对象路径仍使用实例／空间 owner／space／version 隔离。个人存储解析不扩大；绑定中不能撤销分配，不支持原地迁移。

## 封存

归档保留长期 SpacePermission；scopeRole 将原可见角色降为只读视图，但不扩大读取范围。新写入、目录变更、移动、分享创建及上传最终提交复核 Team。撤销所有团队分享、AccessGrant、HandoffGrant、临时申请；Agent 标为归档暂停并撤销，恢复不自动激活。

上传记录 ActorID。已归档上传的原操作者可取消并释放暂存与预留容量，这不是重新授予文件编辑权。进行中传输按现有权限检查点终止；已发生的存储副作用不承诺自动回滚。团队新外部读取只发服务端短期 grant，不发 S3 直链；服务端流在检查点校验权限和 grant，历史对象存储 URL 仍等到原有效期结束。

## 邀请与本地体验

团队邀请绑定内部 user ID 与 team ID，7 天有效，只授予普通成员关系。跨租户者先接受原租户邀请，随后接受团队邀请。账号开通和 24 小时激活沿用原权限合同，管理员不能接管已有用户 Key。没有外部邮件与公开邀请注册。

--demo --governance 显式启用隔离团队体验，直接初始化真实服务数据；仅回环临时数据库，允许已知 admin/custodian/member persona。切换撤销旧浏览器 session 并签发新 session。正式 serve 没有 demo 入口。

## 验证与回滚

复用 Go race、httptest、离线 S3/OIDC fixture 与 evidence runner。测试完整邀请、连接归属、团队空间实际传输、旧接口责任人保护、归档中止与恢复不复活授权；客户端另存真实浏览器证据。

升级前使用已有备份命令保留实例密钥、元数据和对象索引。增量字段不删除；有归档／分配状态写入后不能运行忽略新规则的旧二进制。回滚只恢复完整升级前备份，或前向修复；不自动反向删列。
