# 登录 Key 管理

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


网页“Key 管理”支持复制真实 Key、再次复制、显示/隐藏、用途名称修改、本人轮转、禁用/启用，以及吊销/删除多余或已失效 Key。禁用会结束会话但保留密文，之后可再启用。不能对最后一把有效登录 Key 做禁用、吊销或删除。仅所属用户可取回或轮转；团队或平台管理身份不提供他人的 Key。目录仅返回元数据，密文与哈希不进入列表、日志或审计。删除会擦除可取回密文并失效该 Key 换发的会话。

- `POST /api/v1/key-directory/{id}/reveal`：取回本人有效 Key；响应 `key`，不缓存。
- `POST /api/v1/key-directory/{id}/rotate`：请求 `confirm: true`；原子生成后继，响应 `id`、`key`、`name`、`user_id`、`expires_at`。不自动重试。
- `POST /api/v1/key-directory/{id}/disable`：请求 `confirm: true`；暂停登录并结束该 Key 的会话，密文保留。最后一把有效 Key 返回 `last_active_key`。
- `POST /api/v1/key-directory/{id}/enable`：重新启用已禁用 Key；不恢复旧会话。
- `POST /api/v1/key-directory/{id}/revoke`：请求 `confirm: true`；本人吊销。最后一把有效 Key 返回 `last_active_key`。
- `POST /api/v1/key-directory/{id}/delete`：请求 `confirm: true`；吊销并归档、擦除密文。
- `POST /api/v1/files/batch/restore`、`POST /api/v1/files/batch/purge`：回收站批量操作，最多 100 项；永久删除仍需 `X-Confirm-Permanent-Delete: yes`。
- `POST /api/v1/spaces/{id}/trash-policy`：设置 1–365 天自动清理。
- `GET /api/v1/key-directory`：额外返回 `can_copy`、`can_rotate` 和 `copy_unavailable_reason`。

新 Key 使用实例密钥加密持久化，继续用哈希验证登录。备份仍需保管实例密钥。历史只有哈希的 Key 无法还原；用原 Key 登录一次可补齐加密保存，或本人轮转后使用新 Key。恢复流程的旧哈希记录同样在新密钥首次登录后支持取回。

本人轮转不延长有效期、不改变绑定用户与团队网盘权限。旧 Key 和其他使用它的登录会话立即失效，发起轮转的浏览器会话保持登录。CLI 与旧管理员轮转接口仍按原规则失效全部旧 Key 会话。Agent 授权和恢复码不在本次变更内。

验证：`python3 scripts/check.py -- go test -race ./internal/service ./internal/fixtures`。浏览器集成证据在 cli/drivebridge/web 中生成。没有启用真实部署。
