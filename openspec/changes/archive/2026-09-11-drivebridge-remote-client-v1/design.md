## Context

旧本机模式保留。新增 Go SDK 远程模式，限定 token 与本机目录，HTTP 传字节，MCP 传元数据与稳定引用。

## Decisions

- 内部用户关联 `(issuer, subject)`，不按邮箱合并。每次请求从会话/token 得到用户，不接受客户端覆盖。
- OIDC provider 每部署一个。PKCE、state、nonce、issuer、audience、签名和有效期均验证。OneDrive OAuth 与登录分离。
- Agent token 只显示一次，保存摘要，默认 read/7 天，可限制空间、目录和操作。不能管理连接或其他 Agent。
- 空间存储绑定不可变。对象 key 按实例/用户/空间/版本生成；内容改动创建新版本，ExpectedVersion 防冲突。
- 分片 8 MiB，先写私有暂存目录，再写 S3 并复读验证 SHA-256，最后事务提交版本和配额；重复 complete 返回原回执。
- 元数据用 pure-Go SQLite/GORM，连接凭据 AES-GCM 加密，实例密钥独立保存。恢复缺失密钥时不得生成新密钥掩盖错误。
- OneDrive 使用按连接隔离的 Graph/OAuth 客户端，不在服务端切换 rclone 全局配置。旧本机 rclone 流程保持不变。
- 临时下载授权只经受认证 HTTP 提供给宿主；MCP 返回固定版本引用，SDK 敏感 URL 不序列化。有效期不等于可立即撤销。
- 所有传输恢复使用原 operation。回存/导入不发起生成、发布或 canonical 采纳。

## Interfaces

HTTP `/api/v1`、OIDC `/auth`、可选 MCP `/mcp`。文件字节经 HTTP，远端不读取客户端磁盘。网页与本机 Agent 使用同一 API。工具 schema 由真实注册表生成。

## Verification

Go fake OIDC（真实 RSA 签名和 PKCE）+ HTTP S3 fixture、SDK、CLI/MCP 回归；Vitest 与 Playwright 覆盖真实登录、上传、恢复、版本、权限、撤销、拖拽与移动端。证据在各 owner 的 temp/integration-test-runs/。

## Migration / Rollback

全部 pre-1.0 增量扩展；保留原本机命令、引用和配置。关闭新服务/远程模式回滚，保留数据与回执。部署前验证真实 issuer、S3、OneDrive 和 Provider 区域；不以本地 fixture 宣称生产可用。

## Risks

首版为单实例。S3 连接验证探测可达性，实际写入由文件流程验证。对象写入后元数据失败可能留下可恢复的暂存对象；未完成记录保留用于恢复。公网/私网地址需要明确配置，默认不开放公网。
