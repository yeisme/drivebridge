# DriveBridge Server

本目录是单仓 `cli/drivebridge` 的内部模块。日常开发从父目录执行 `make dev` / `make build` / `make test`；统一规格见 `../openspec`，开发说明见 [统一入口](../docs/development.md)。下文保留模块使用说明。

单实例、多用户个人文件基础设施。本服务持有 OIDC 会话、用户/Agent 授权、文件版本、存储连接和操作记录；网页由 `cli/drivebridge/web` 拥有，本机传输入口由 `cli/drivebridge` 拥有。当前为 pre-1.0 本地候选，默认关闭，尚未发布部署。

## 本地无账号预览

先在网页项目构建，再从本项目启动**独立的测试程序**：

```bash
pnpm --dir ../web build
go run ./cmd/drivebridge-fixture --listen 127.0.0.1:8787 --web-dir ../web/dist
```

打开 `http://127.0.0.1:8787`，使用测试身份 Alice 或 Bob 登录。这个程序启动本机 OIDC（真实 RSA 签名与 PKCE）、HTTP S3 fixture 和 API，数据写入临时目录；停止后清理。它不访问真实 Ory、微软、S3 或 Provider，不能用于生产。

## 实际服务入口

```bash
go build -o drivebridge-server ./cmd/drivebridge-server
./drivebridge-server serve --state /absolute/private/drivebridge-state
```

未加 `--enabled` 时业务入口关闭，默认只监听回环地址。部署前用只读预检确认 state 一致性、web 构建产物、平台存储与 issuer discovery（不取实例锁、可诊断拒绝启动的恢复场景）：

```bash
./drivebridge-server doctor --state /absolute/private/drivebridge-state \
  --web-dir ../web/dist \
  --issuer https://your-oidc-issuer.example.com
```

完整生产部署清单见 [docs/deployment.md](../docs/server/deployment.md)。显式启用示例：

```bash
./drivebridge-server serve \
  --state /absolute/private/drivebridge-state \
  --listen 127.0.0.1:8787 \
  --origin https://files.example.com \
  --issuer https://your-oidc-issuer.example.com \
  --client-id '<registered-client-id>' \
  --client-secret-file /absolute/private/oidc-client-secret \
  --web-dir ../web/dist \
  --enabled
```

示例域名和路径需要替换。公网反向代理、DNS、TLS 与 OIDC 应用注册由操作者独立配置，程序不自动开放公网。注册精确回调地址 `<origin>/auth/callback`。公共客户端可以不提供 client secret；始终使用 PKCE。

OIDC 每部署一个 issuer，使用 `(issuer, subject)` 关联内部用户，不按邮箱合并。Ory 必须实际提供标准 OIDC（如 Hydra/Ory Network）；Kratos 登录 UI 本身不等于 OIDC issuer。微软登录也应配置经过 discovery 验证的具体 issuer；首版不自动处理 `common` 等多 issuer 入口。网页会话有效期 12 小时，本应用退出会删除本地会话；没有实现上游全局登出事件同步。

开发用回环 HTTP 须显式加 `--development`。若使用 Vite 代理热开发，`--origin` 要设置为实际浏览器 origin；完整离线体验推荐使用上面的同源构建预览。

## 存储与密钥

平台 S3 由操作者通过 CLI 一次性配置；连接配置从 stdin 读取，凭据文件不得进入仓库：

```bash
./drivebridge-server platform-connect --state /absolute/private/drivebridge-state < /absolute/private/platform-s3.json
```

输入字段为 `endpoint`、`region`、`bucket`、`prefix`、`access_key`、`secret_key`，可选 `session_token`。文件由用户的凭据管理流程提供；不要让 Agent 在仓库内拼装真实凭据。首次连接会探测存储可达性，实际写权限在上传时验证。首版不允许原地重绑平台存储，避免旧对象指向另一个桶。

用户可在网页连接自己的 S3，为未绑定个人空间选择存储，或建立绑定新存储的空间。空间绑定后不可改变。默认每空间/文件上限 1 GiB，操作者可通过 `--max-bytes` 调整；当前不是计费或组织配额系统。

生产连接要求 HTTPS 公网端点；默认拒绝重定向、私网/回环数据端点。回环 fixture 仅在显式 development 模式允许。不默认配置 TOS，不自动访问任意 URL。

`instance.key` 是随机生成的 32 字节 AES-GCM 实例密钥，独立存放于私有 state 目录；连接凭据绑定 user ID 加密。`metadata.sqlite` 保存元数据与密文，`instance.id` 保存远程引用来源。密钥缺失而数据库存在时拒绝启动，不生成新密钥掩盖恢复问题。

## OneDrive 连接

网页登录与微软网盘授权互相独立。启用 OneDrive 按需提供 `--onedrive-client-id` 和可选 `--onedrive-secret-file`，注册回调 `<origin>/auth/onedrive/callback`。用户在已登录的网页授权 `Files.ReadWrite` 与 `offline_access`。

服务端每个连接独立保存并串行刷新 OAuth 凭据，Graph 请求固定账号 drive。没有在多用户进程里切换 rclone 的全局配置。旧本机 rclone 模式继续保留。

- 浏览不复制整个网盘；选择文件才创建导入操作。
- 导入绑定源 item/ETag，分片下载后再次检查版本，生成受管 S3 副本。
- 原文件变更或 OneDrive 断开不影响已导入副本。
- 回存使用 operation ID 命名文件，重试先验证同名结果，不创建生成任务。
- Agent 需要明确的 `import`/`export` scope 和指定连接，且仍受空间与目录限制。

微软直链和上传 session URL 只在连接器进程内使用，不进入 MCP、审计或操作回执。真实微软应用/账号与网络区域尚需单独验收。

## HTTP、SDK 与 MCP 合同

浏览器用 HttpOnly、SameSite cookie；写请求同时要求固定 Origin 和会话 CSRF token。Agent 使用 `Authorization: Bearer <scoped-token>`。用户 ID 不接受请求参数覆盖，Agent 不能创建其他 Agent 或管理连接。

| 能力 | HTTP 入口 |
|---|---|
| 用户、空间、能力 | `GET /api/v1/me`、`/spaces`、`/capabilities` |
| 文件与版本 | `GET /api/v1/files`、`/files/{id}`、`/files/{id}/versions` |
| 上传 | `POST /api/v1/uploads`；`PUT /operations/{id}/parts/{index}`；`POST /operations/{id}/complete` |
| 操作恢复 | `GET /api/v1/operations`、`/operations/{id}`；`POST /operations/{id}/resume`、`/cancel` |
| 下载与私有交接 | `GET /api/v1/files/{id}/content?version=...`；`POST /files/{id}/access` |
| 回收站 | `POST /api/v1/files/{id}/trash`、`/restore`；`DELETE /files/{id}` |
| 连接 | `GET /api/v1/connections`；`POST /connections/s3`、`/connections/onedrive`、`/connections/{id}/check`、`/disconnect` |
| OneDrive | `GET /api/v1/connections/{id}/items`；`POST /connections/{id}/import`、`/export` |
| Agent 与审计 | `GET/POST /api/v1/agents`；`POST /agents/{id}/revoke`；`GET /api/v1/audit` |
| 原生 MCP | `/mcp`，Streamable HTTP，手动配置限定 token |

除 `/auth`、`/healthz`、`/mcp` 外表中相对路径均带 `/api/v1` 前缀。JSON 成功体为 `data`，失败体为 `error.code`。上传创建请求包含 `space_id`、相对 `path`、`size_bytes`、完整小写 `sha256`；header 使用稳定 `Idempotency-Key`。创建新版本还需 `file_id` 和 `expected_version`。分片大小 8 MiB、index 从 0 开始，最后一片可较小；不能用不同内容替换已接收分片。

永久删除要求用户会话和 `X-Confirm-Permanent-Delete: yes`，只清理回收站内文件。Agent 的 delete 权限只允许软删除/恢复。

Go SDK 位于 `sdk/`，上传从 `io.ReadSeeker` 计算摘要、查询原操作并补齐缺片；下载校验摘要。私有下载授权需明确 version ID，默认五分钟 S3 presigned URL。SDK 的 `Access.URL` 不参与 JSON 序列化；MCP `prepare_reference` 只返回稳定引用。持有外部直链者可能在到期前继续访问，撤销 Agent token 不等于即时撤销已签发 S3 URL。

原生 MCP 从同一个工具注册表生成 schema，提供 `create_upload`、`complete_upload` 和查询/恢复入口。文件字节通过 HTTP；纯远程 MCP 不读取客户端本机路径。安装本机适配器后，Agent 可使用 `upload` / `download` 工具，无需安装各创作产品 CLI。

## 恢复与备份

元数据通过 GORM 事务保留状态和配额。S3 写入后复读 SHA-256，成功后才发布版本；重复完成返回同一版本。成功或取消后清理本机暂存片；远端暂存对象若未完成元数据提交，会保留其操作记录供恢复，不自动清理未知云对象。

关闭服务后，备份 CLI 取得同一实例锁，复制元数据与实例 ID：

```bash
./drivebridge-server backup --state /absolute/private/drivebridge-state --output /absolute/new/backup-directory
```

实例密钥必须使用独立的秘密备份渠道保管。恢复时，在新的权限 0700 state 目录放入备份的 `metadata.sqlite`、`instance.id` 与原 `instance.key`，再启动服务；不要覆盖运行中的实例。仅恢复元数据时，缺失的上传暂存片会被标为需要原文件重传；已完成 S3 文件无需重新上传。S3 字节备份/版本策略由存储操作者管理，元数据备份不复制所有对象。

## 本地验证

```bash
python3 scripts/check.py
python3 scripts/check.py -- go vet ./...
python3 scripts/check.py -- env CGO_ENABLED=0 go build ./cmd/drivebridge-server
openspec validate drivebridge-personal-service-v1 --strict
```

每次证据位于 `temp/integration-test-runs/`。测试覆盖真实 OIDC 签名/PKCE、双用户隔离、HTTP S3、版本与配额、取消和重启、加密连接、OneDrive 合同、限定 token、原生 HTTP MCP 和备份恢复。真实 Ory/微软/S3/Provider、部署与发布分别报告，不以 fixture 替代。

`platform-connect` 与 `backup` 支持 `--json`、`--agent`、`--events`、`--explain` 输出；serve 的生命周期信息保持在 stderr。恢复时数据库存在但实例密钥或实例 ID 缺失会拒绝启动，避免静默改写稳定引用。

## 个人 Agent 闭环增量

新增可选 caller/task_ref 来源标签、固定版本引用查询、检索过滤与原生 MCP search/trash/restore。字段、权限与历史兼容说明见 [文件闭环合同](../docs/server/personal-agent-loop.md)。本轮仍为本地候选。

用量、图表、扫描与安全审计见[使用说明](../docs/server/insights-audit.md)。
