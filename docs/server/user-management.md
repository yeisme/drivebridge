# 用户目录与账号管理

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


用户目录支持状态、角色、团队筛选，以及用户/Key 名称或标识查询，默认每页 50 条。Key 详情只包含名称、ID、有效期和状态，平台管理身份不会获得他人的登录密钥明文；无权查看时显示信息受限，不显示假零值。

接口位于 `/api/v1/governance/users`：GET 列表、GET `/{id}` 详情、GET `/{id}/impact?action=freeze` 操作影响、POST `/{id}/action` 确认执行。写入请求包含 action（freeze/unfreeze/archive/restore）、expected_revision、confirm，以及 confirmation_name（目标用户名）或 proof（本人 Key 验证）。用户名不替代登录认证。全局账号操作仅平台管理员可用；不能停用自己；最后有效管理员或数据负责人可由最高管理员在确认清单后接任并继续。

冻结停止访问并失效已有会话和临时授权，解冻后用户需重新登录。删除为归档，不永久删除文件和审计，但吊销 Key、Agent 与分享；恢复归档后保持冻结，之后可解冻并由本人恢复凭据。旧 account-status 不能绕过归档状态或交接约束。

User 的 archived/revision 为 GORM 增量字段。未升级服务时客户端保留基础目录并明确提示能力不可用，不模拟冻结或删除成功。

验证：`python3 scripts/check.py -- go test -race ./internal/service ./internal/fixtures`。浏览器验证在 cli/drivebridge/web 的 tests/users-management.spec.ts，证据归各 owner 的 temp/integration-test-runs。

## 最高管理员接管

影响响应提供 takeover_allowed、handoffs、takeover_digest。界面逐项列出缺失的职责，最高管理员勾选后提交 takeover 与原摘要。服务在事务内重新计算清单、授予执行者相应职责、再完成冻结或归档。摘要变化返回 409。默认操作与旧接口不隐式接管，也不因此扩大所有管理身份的写权限。

## 本人新建登录 Key

新增兼容接口 `POST /api/v1/key-directory`，请求为 `name` 和可选 `days`（0 表示长期有效，上限 3650 天）。仅已认证的用户可为本人创建；不接受指定其他 owner，也不允许 Agent 或服务身份调用。响应包含新 Key，使用 `Cache-Control: no-store`；服务端沿用密文存储和审计目标记录。旧 Key、会话及现有接口不变，无数据库迁移。删除该新增入口可以回退功能，已签发 Key 仍由既有轮转／吊销流程管理。
