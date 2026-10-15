# 多租户、团队与管理授权

## 自托管管理员直接读写（2026-10-12）

部署者可显式使用 `drivebridge-server serve --enabled --admin-direct-write`，让人类管理员直接读写其已有管理范围。平台管理员覆盖平台可管理租户，租户和团队管理员仍限于各自范围；停用用户、停权角色、停用租户和归档团队仍受原有校验。此开关不把 Agent 令牌升级为管理员，也不授予第三方服务身份额外权限。默认不开启时，下文的管理只读与审批流程保持原样。

开启后，平台管理员即使没有租户成员记录，也能进入所选有效租户的工作区。管理员自己创建的个人空间按本人空间处理；管理他人空间的连接凭据、身份和 Agent 签发仍使用各自的权限边界。Web 从服务端的 `admin_direct_write` 读取状态，不通过隐藏按钮替代服务端授权。

## 移除历史空间

停止 HTTP 服务及 worker 并备份后，使用精确空间 ID：

```bash
drivebridge-server --state /absolute/private-state backup --output /absolute/new-backup
drivebridge-server --state /absolute/private-state spaces retire --id SPACE_ID --offline --json
```

命令在事务中移除正常查询可见的空间，取消 `awaiting_upload`、释放预约、撤销该空间的 Agent 授权和分享。未知执行中的操作会拒绝整批移除，必须先对账。重复执行不会创建第二次移除事件。历史版本字节、元数据和审计保留用于恢复，不执行云端物理清除。使用包含本能力的服务版本继续运行；旧二进制不理解移除标记，不能直接降级打开新状态。

DriveBridge 可在单个部署内管理多个隔离租户。平台最高管理员读取全平台，租户管理员读取本租户，team 管理员读取本团队空间；个人空间不会因为加入 team 而暴露给 team 管理员。凭据明文始终不属于管理读取内容。

## 启用与迁移

此功能必须由部署操作者显式启用，不把原 `admin` 标记自动升级。先停止服务并使用现有备份命令备份元数据、实例标识及密钥，再在服务端项目执行：

```bash
go run ./cmd/drivebridge-server --state /path/to/private-state governance bootstrap --user USER_ID --json
```

`USER_ID` 是已有内部用户 ID，不是邮箱或密钥。初次启用将既有用户及空间归入默认租户；后续同一命令只负责明确任命或恢复平台管理员，不重新接纳已移除成员。文件 ID、版本、引用与存储对象位置不变。启动服务后，Web 显示租户切换和管理授权中心。

启用前旧部署接口行为保持兼容。启用后，原用户密钥签发、轮换、吊销等 Web/MCP 管理写入口返回 `account_recovery_required`，避免用用户凭据冒充本人；新账号走激活，已有账号走恢复或冻结。此安全收紧没有保留不受限的旧入口。部署侧操作者持有数据库与密钥，仍属于独立的主机信任边界。

新租户产生写入后不得直接使用旧版本服务端打开数据库。回滚只能恢复经过验证的完整备份恢复点，或进行前向修复；不能声称通过关闭开关即可无损回退。旧字段本次保留，普通弃用至少经过一个发布周期。

统一身份映射由部署侧维护，凭据不进入输出：

```bash
go run ./cmd/drivebridge-server --state /path/to/private-state governance identity-link \
  --issuer https://idp.example.com --subject SUBJECT --identity-tenant ORG \
  --user USER_ID --tenant TENANT_ID [--agent-id AGENT_ID] [--cutover] --json
```

同一 issuer/subject/身份组织三元组不可改指他人；`--cutover` 要求本租户全部活跃成员已映射，切换后旧凭据以 `unified_login_required` 拒绝。能力发现与六个错误码合同见[公共合同与 owner](contract-map.md)。

## 账号与团队

账号统一，成员关系、个人空间及角色按租户隔离。平台管理员可开通空账号，获得一次性的 24 小时激活码；用户在登录页生成本人登录密钥并激活，再接受租户邀请。激活码不能用于恢复已有账号。

平台管理员创建租户并指定租户管理员；租户管理员创建 team 并任命 team 管理员；team 管理员维护已有租户成员。数据负责人及备份审批人按空间指定。协作角色为只读、编辑、数据负责人；管理身份不自动取得编辑权。个人空间归成员，团队空间独立归团队，首批只绑定平台存储。

任何人不得自批或自授权限。空间负责人或有权上级可授权其他成员；个人数据的普通审批只由本人完成。团队申请可由数据负责人、租户管理员或平台管理员代批。只有平台最高管理员有应急权。

移除成员、降低权限、停用租户或冻结账号后，新的访问被拒绝，相关未完成授权被撤销，数据保留。重入不会恢复旧临时授权。租户管理员不能冻结全局账号或重置全局登录凭据。

## 申请与执行

申请默认 24 小时过期。范围许可支持限定目录内的一般写入，默认 1 小时、最长 8 小时；不能用于覆盖既有文件、删除或分享。移动会检查源路径与目标路径。

具体清单支持移入回收站、恢复、永久删除、移动、使用空间内固定来源版本覆盖，以及生成一天有效的固定版本下载分享。覆盖前先准备来源文件，审批固定目标版本及来源版本。清单批准后 1 小时内由申请人确认执行，不能自批，不能在批准时自动执行。版本改变后失败，不能悄悄修改新版本。永久删除仍要求文件已在回收站且没有阻塞引用。

清单逐项记录状态和错误。重复执行已领取的许可会拒绝；部分失败或重启中断不能自动重跑成功项目，应基于当前版本申请剩余操作。撤销不等待存储 I/O，正在执行的项目在下一权限检查点停止，已提交修改不会自动回滚。

应急要求最高管理员重新输入当前密钥，或通过带可验证 `auth_time` 的近期 OIDC 认证。固定 15 分钟，限定目录与动作，立即记录和通知。扩大范围或续期必须重新启动。OIDC Provider 不提供认证时间时不能仅凭 SSO 回跳认定已经重新验证。

## 通知、读取与恢复

站内通知持久保存，区分未读与已读，不声称用户已收到外部消息。查看按操作者和空间的 30 分钟窗口汇总，下载及 ZIP 导出每个请求单独记录。申请、审批、撤销、过期、执行结果、应急及恢复均保存记录。关键事件与通知在同一数据库事务中写入，保存失败时不开始管理操作。

管理下载使用服务端流，不发放可脱离管理上下文的临时存储直链。流每 256 KiB 或一秒重新检查权限；撤销不能收回已经下载的副本。普通所有者原有临时链接继续遵守已有有效期合同。

本人可生成一次性恢复码。没有恢复码时，在登录页提交本人生成的新密钥及领取证明的哈希绑定，平台管理员通过独立渠道核验本人和请求 ID，只批准核验、不取得新密钥或领取证明。领取后旧密钥、会话、Agent 与未完成授权失效。

人工恢复先暂停平台、租户、team 管理及空间审批角色，另一位最高管理员重新确认。人工核验存在误认风险；不把这项流程声称为自动身份验证。最后一位最高管理员失去凭据时，由部署操作者执行本机恢复与上述显式角色引导，不提供 Web 自批后门。

## 公共接口与客户端

既有 HTTP 数据接口保持路径和响应主体，新增可选 `tenant_id`、`team_id`、`actor_id` 等字段。`GET /api/v1/me` 返回 `governance_enabled` 和当前租户；旧 `admin` 字段只是兼容投影，服务端实时读取角色。

- `GET /api/v1/governance`：当前租户的可见空间、角色、团队、申请、通知、审计和本人邀请。
- `POST /api/v1/governance/{action}`：`tenants`、`tenant-status`、`invite`、`accept`、`remove-member`、`member-role`、`teams`、`team-member`、`spaces`、`permission`、`requests`、`approve`、`reject`、`revoke`、`execute`、`emergency`、`read-notification`、`provision`、`platform-role`、`recovery-approve`、`restore-privileges`、`account-status`、`team-invite`、`team-invite-cancel`、`team-invite-accept`、`team-invite-decline`、`team-invite-resend`。
- `POST /api/v1/governance/recovery-code`：本人重新验证后生成恢复码，仅显示一次。
- `POST /auth/enroll`、`POST /auth/recovery/request`、`POST /auth/recovery/complete`：本人激活或领取恢复，要求同源请求。
- `GET /api/v1/files-export?files=ID1,ID2`：在当前允许空间中流式生成 ZIP。

HTTP 使用 `X-DriveBridge-Tenant` 与 `X-DriveBridge-Space` 选择上下文；媒体与原生下载使用相应的 `tenant_context`、`space_context` 查询参数。它们不授予权限，不能替换登录身份。SDK `client.Scoped(tenant, space)` 返回独立副本，避免并发调用共享上下文。

没有安装 CLI 的 MCP 客户端通过 `tools/list` 获取可选 `tenant_context`、`space_context`，通过 `drivebridge://capabilities` 获取 HTTP 数据传输合同。Agent token 保持原租户与空间范围，不因签发人的管理员身份获得管理权；`approval_required` 需由人进入服务端 Web 授权中心处理。MCP 参数不是远端可读的本机路径。

## 验证

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
```

测试覆盖租户隔离、团队与个人边界、自批拒绝、范围撤销、固定版本覆盖与清单重放、应急过期、人工恢复暂停特权、激活码一次性、审计故障、流式凭据撤销及长任务撤销竞争。Web 和远端适配器在各自 owner 保存验证证据。本次没有访问真实云账号或部署。
