## ADDED Requirements

### Requirement: 本人重复取回登录 Key
服务 SHALL 仅向当前认证用户返回其本人有效 Key 的明文，目录 SHALL 只返回元数据与 can_copy、can_rotate、copy_unavailable_reason。管理员、团队上下文、Agent 和服务身份 SHALL NOT 扩大该权限。

#### Scenario: 再次复制
- **WHEN** 本人多次调用 POST /api/v1/key-directory/{id}/reveal
- **THEN** 服务重新鉴权并返回相同有效 Key，响应不缓存且审计不含密钥

#### Scenario: 旧 Key 无可取回密文
- **WHEN** 已有 Key 只有哈希
- **THEN** 读取明确返回 key_material_unavailable；原 Key 成功登录后可再次取回，或通过本人轮转生成新 Key

#### Scenario: 无效或他人 Key
- **WHEN** 调用者请求他人的 Key，或本人 Key 已过期、归档、吊销
- **THEN** 服务拒绝返回密钥

### Requirement: 本人确认轮转
服务 SHALL 在本人明确确认后原子轮转 Key，保留身份、权限和有效期，旧 Key 立即停用，最多产生一个有效后继。

#### Scenario: 当前会话轮转
- **WHEN** 本人在当前浏览器确认轮转来源 Key
- **THEN** 当前浏览器仍能访问，其他来源于旧 Key 的会话和客户端失效，新 Key 可再次取回
