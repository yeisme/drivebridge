## Context

旧实现只有不可逆哈希，前端复制 ID 不能登录。参考 [New API 的 GetTokenKey](https://github.com/QuantumNous/new-api/blob/main/controller/token.go)，按当前用户与 token ID 精确读取，目录不返回明文。

## Decisions

复用实例 AES-GCM、GORM AutoMigrate 和既有审计。SealedKey 可空且 json 隐藏。导入、签发、激活及轮转时加密；旧 Key 与恢复流程生成的哈希记录在成功登录时补齐。POST reveal/rotate 使用现有认证及会话 CSRF，拒绝 Agent、服务身份与跨用户访问。响应 no-store，审计只记录动作与资源 ID。

轮转先在事务内读取有效旧 Key，条件更新 revoked 防止多个后继，再创建加密新 Key。当前 session 的 origin_key 仅在与旧 ID 一致时迁移；其他会话自然失效。本人轮转保留到期时间及全部身份字段，不允许借机延期或升级。

## Migration and compatibility

数据库为增量兼容迁移，旧记录 SealedKey 为空，旧哈希认证继续有效。新 HTTP 方法和目录字段均为增量，无接口移除，无废弃窗口。用户明确更改此前只保存哈希的产品策略；不批量找回或更改旧 Key。新接口失效时可撤回前端/服务代码，保留密文列；哈希与旧认证仍可用。

## Validation

Go 测试覆盖再次读取、管理员跨用户拒绝、治理模式、旧 Key 登录补齐、失效与篡改密文、轮转确认、权限和有效期继承、双会话及并发唯一后继。fixture 验证体验入口在轮转后仍能登录。证据用 scripts/check.py 生成，禁止记录密钥响应。
