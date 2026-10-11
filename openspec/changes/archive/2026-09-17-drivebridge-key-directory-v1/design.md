# Key 目录边界

新增 GET /api/v1/key-directory 返回当前可见 Key 的公开元数据；新增 POST /api/v1/key-directory/{id}/label 仅修改 Name。无数据库迁移，不改变旧 users 列表／轮转／撤销合同。客户端改用新投影以获得 user_name、created_at、current。

所有请求经现有认证、CSRF、context binding 与 audit wrapper。Agent 与 service identity 不可访问登录 Key 目录。普通身份和治理模式严格用 p.UserID 筛选；非治理管理员保留旧管理员全局范围。数据空间的 DataOwner 不作为凭据身份，避免管理空间授权扩散到凭据。

标签 trim 后限制 1–80 字，不接受换行和 NUL；归档 Key 不可修改。Owner、TokenHash、权限、有效期保持不变。current 从当前 Bearer Key 或浏览器 session 的 OriginKey 得出。返回中不含 TokenHash 或明文。

以 Go race 测试校验身份映射、无凭据泄露、跨用户拒绝、改名不变更权限、禁用空名称、Agent 拒绝，以及治理启用后的管理员范围收敛。Web 通过真正标签接口进行浏览器验证。回滚客户端使用旧元数据并撤销新路由即可；持久化标签保留且兼容旧版本。
