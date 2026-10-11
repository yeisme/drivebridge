# 设计

User 增加可空 display_name、avatar_ref 字符串字段，由既有 GORM 自动迁移扩展。GET /api/v1/me 的 user 保留 id/name 等字段，新增字段可忽略。

PATCH /api/v1/me 仅允许本人浏览器会话，复用 authenticate 的 Origin/CSRF 检查。显示名称 trim 后按 Unicode 计数 1–80，不含控制字符；无更新字段返回校验错误。JSON body 上限 4096 bytes。头像允许空串或 null 清空，其余只允许 file:<file-id>:<version-id>。

头像候选取本人个人空间，包含本人 active membership 的 data owner，排除团队、加密、回收站与待审核文件；MIME 白名单为 PNG/JPEG/WebP/GIF，大小 1 byte 至 2 MiB。版本与文件、owner 必须匹配。GET /me/avatar-options 最多返回 30 项，查询长度最多 80 字符。GET /me/avatar 每次重新校验资源，继续调用媒体读取服务，不直接暴露对象地址或 URL。

不接受凭据令牌、Agent、服务身份或分享访问来写资料。存储查询通过 GORM。无删除字段、无数据回填；回滚客户端隐藏编辑入口，服务端保留已写数据，不改变旧客户端 name 语义。
