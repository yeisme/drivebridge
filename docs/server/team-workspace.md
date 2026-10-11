# 团队服务与工作区合同

本服务维护团队生命周期、成员、用户绑定邀请、空间权限、存储分配和审计。Web 六页团队工作区归 cli/drivebridge/web。所有状态经 GORM 和既有治理服务写入，不新增 BFF 或组织平台。

## 读取接口

以下接口都需要人类用户身份及当前租户上下文。Agent 和服务身份不能读取团队管理目录。

- `GET /api/v1/governance/teams`：支持 `filter=all|joined|managed|archived`、`q`、`page`、`limit`。默认 50 条，最多 100；响应含 `items`、`next_page`、`enabled` 和 `can_create`。
- `GET /api/v1/governance/teams/{id}`：团队说明、归档状态、revision、角色、实际可见容量、actions、权限、近期文件、Agent、待审批申请及连接元数据。
- `GET /api/v1/governance/teams/{id}/members|spaces|activity|invitations`：分页子资源；只返回当前身份可见内容。
- `GET /api/v1/governance/teams/{id}/impact`：归档／移除影响，支持指定 user_id、space_id、role；返回 revision、阻塞原因及相关资源数量。
- `GET /api/v1/governance/team-invitations`：本人等待处理的团队邀请，含对应租户成员状态；不授权他人代为接受。

`actions` 为 `{allowed, reason}`。它只供客户端展示，不能作为后端授权凭据。页码超出结果返回空列表，不回退到其他租户。团队权限变化后客户端需重新读取。

## 写入与权限

沿用 `POST /api/v1/governance/{action}`。新增 `team-update`、`team-archive`、`team-restore`、`team-leave`、`team-invite`、`team-invite-cancel`、`team-invite-accept`、`team-invite-decline`、`team-invite-resend`、`team-storage-assign`、`team-storage-unassign`；这些动作要求 `expected_revision`。陈旧版本返回 `version_conflict`（409）。已结束的邀请续期或拒绝返回 `invitation_not_pending`（409）。`teams`、`team-member`、`spaces`、`permission` 仍支持旧请求，新客户端也发送版本号。

团队管理身份不自动拥有数据编辑权。租户管理员创建团队／网盘、任命管理员、分配连接、归档和恢复；团队管理员维护普通成员、邀请本租户成员及修改团队资料；本人可退出但不能绕过最后管理员／数据负责人保护。数据负责人或有权上级授权其他成员，不能自授权限。

新用户继续由平台管理员开通空账号，本人使用 24 小时激活码生成自己的登录 Key。团队邀请绑定用户和团队，七天有效；先接受租户邀请再接受团队邀请。接受仅创建普通成员关系，不自动授予网盘权限。绑定用户可拒绝未过期邀请；管理员可对同一邀请 id 续期七天（含已过期但仍为 pending 的行）。本人收件箱投影含 `next_step`。只发送站内通知，不自动邮件，不提供开放链接注册。

## 存储分配

团队可使用平台 S3，或由租户管理员显式分配其本人在当前租户持有的 S3／COS 兼容连接。凭据仍属于原连接 owner，不返回或复制到团队。其他人的私人连接不可分配。

TeamStorage 记录团队、租户、连接和分配人。storageForSpace 对每个具体网盘验证空间 owner、连接 ID 和分配记录，再解封原连接 owner 凭据。上传、完成、下载、范围读取和容量扫描使用同一路径。对象键继续包含实例／空间 owner／space／version；两个团队共用连接不意味着共用对象目录。

绑定中取消分配返回 `storage_in_use`（409），不删除数据、不自动改换后端。已断开的连接返回不可用；OneDrive 是导入／导出能力，本轮不能作为团队受管网盘。已有网盘不原地迁移。

## 归档

归档是只读封存：保留成员原本可读的网盘，拒绝后续写入。服务撤销团队分享、AccessGrant、HandoffGrant、临时申请并暂停 Agent；恢复不复活这些能力。归档不授予以前没有的读权限。

目录修改、移动、分享及上传提交在数据库事务内再次检查归档状态。外部 I/O 不持有治理锁；归档不等待正在上传的对象传输，上传在后续检查点拒绝发布元数据。已发生的存储副作用不承诺自动回滚。原上传操作者可取消未完成任务释放暂存和配额。

团队新发放的临时内容访问走服务端 grant 流，范围读取与流式读取持续检查权限及 grant。历史已发出的 S3 直链可能有效至原到期时间，已下载的副本无法撤回。

## 迁移、回滚与体验

Team 增量增加 description、archived、revision 和 updated_at，新增邀请和分配表。已有团队默认活跃，原有平台团队空间补齐兼容分配。文件 ID、版本与稳定引用不变。旧治理入口保留，但也受最后负责人保护。

升级前保存完整状态与实例密钥。产生归档或新连接分配后，不可运行忽略新规则的旧服务。回滚必须恢复升级前完整备份，或前向修复；不通过删列或关闭治理开关无损回退。

本地显式运行以下命令可体验真实角色：

```bash
go run ./cmd/drivebridge-fixture --listen 127.0.0.1:10414 --web-dir ../web/dist --demo --governance
```

仅回环、临时状态；admin/custodian/member 为固定允许的模拟身份。切换会话撤销旧会话，保持开发代理兼容。正式 serve 没有该入口，不修改真实租户、凭据或基础设施。

## 验证

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
```

团队专项覆盖旧接口保护、版本冲突、指定用户邀请、分页和隔离、私有连接分配、实际对象上传／下载、归档中的上传提交拒绝、预留容量释放及恢复不复活授权。全部证据在本项目 temp/integration-test-runs；Web 的浏览器验收在 client owner 保存。

本轮通过证据：服务全量 race 回归在 `temp/integration-test-runs/20260917T022755Z-c4891d80/summary.json`；最终团队／旧接口专项在 `temp/integration-test-runs/20260917T023815Z-464743e3/summary.json`；上传 I/O 中归档且禁止提交的专项在 `temp/integration-test-runs/20260917T022505Z-09f62009/summary.json`。`go vet ./...` 与 `CGO_ENABLED=0` 构建通过。未访问真实云账号或修改真实部署。
