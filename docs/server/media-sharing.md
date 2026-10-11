# 媒体、分享与创作交接合同

此能力增量扩展现有个人文件服务。文件与版本、权限、存储、受控传输继续归 DriveBridge；生成任务、模型、费用和审核仍归 Eikona／Scaena。

## API

- `GET /api/v1/browser`：space_id、path、q、cursor；返回受管目录项、下一页与总数。显式 Directory 记录保留空目录。
- `POST /api/v1/directories`、`POST /api/v1/files/{id}/move`：目录创建与文件移动；移动要求 expected_version，保留文件身份与旧版本。
- `POST /api/v1/directories/copy|move|trash`：按文件夹路径前缀复制、同空间移动、回收站。copy 须 `Idempotency-Key`，目标须受管 S3，超过 1000 个文件为 `directory_too_large`。不能把目录移动到自身或子路径。网页不得自行翻页递归复制。
- `POST /api/v1/directories/export`：把文件夹前缀下的文件回存到已连接的 OneDrive。须 `Idempotency-Key`。OneDrive 不是空间对象存储；generic rclone 不是此接口目标。
- `POST /api/v1/files/{id}/copy`：把固定源版本复制成新文件。请求含 `target_space_id`、`target_path`、`expected_version`、可选 `conflict=rename|fail`（默认 rename）；`Idempotency-Key` 必填。目标须为受管 S3。同范围 CAS 命中则 `blob_reused` 且零字节传输。绝不覆盖已有路径。与 `POST /api/v1/archive` 不同：copy 是 Finder 复制，archive 仍是产物归档且排除秒传。复制到 OneDrive 走既有 `POST /connections/{id}/export`。
- `GET /api/v1/files/{id}/media?version=...`：固定版本的安全 inline 读取，支持单段／后缀 Range。原 content 下载路径保持兼容。
- 同一媒体入口可选 `preview=interactive`：仅用于已授权、未回收、固定版本且不超过 2 MiB 的 `text/html`。响应使用 `sandbox allow-scripts` 与限制资源/网络/表单的 CSP；不授予同源、顶层导航、弹窗权限。未传参数仍是禁脚本静态预览，SVG 不获得交互权限，原 content 下载保持 attachment。无需迁移数据库；回退客户端入口即可停止使用该增量能力。主应用 CSP 不放宽。
- `GET/POST /api/v1/files/{id}/previews`、`GET /api/v1/previews/{id}/content`：按版本和 variant 复用预览。转换引擎在 sdk/mediapreview 中与本机 owner 共享。
- `GET/POST /api/v1/shares`、`POST /api/v1/shares/{id}/link`、`POST /api/v1/shares/{id}/revoke`：分享管理，创建可携带 Idempotency-Key；重复键须有相同请求摘要。
- `/api/v1/shared/{id}`：接收方通过 open 换取专用 HttpOnly cookie；每次列表、内容或缩略图请求重新校验分享状态与接收身份。
- `POST /api/v1/users/{id}/archive`、`POST /api/v1/agents/{id}/archive`：吊销并归档；最后一个有效管理员受到保护。租户治理开启后继续应用其独立审批门。
- `GET /api/v1/files/{id}/usage`：永久删除影响，保护活动分享与创作交接引用。
- `/api/v1/creative/connections`、`/api/v1/creative/submit`、`/api/v1/creative/receipts`：用户的领域消费连接、提交交接与原任务投影。`POST /api/v1/creative/receipts/{id}/save` 只回存原任务确实返回的产物。

## 保护与恢复

预览固定文件版本，HTML 等文本按沙箱读取。解密空间范围读取复用内容解密路径；缩略图缓存使用独立派生密钥加密。加密素材的转换只在内存暂存区处理，当前上限 32 MiB；没有该暂存区时明确拒绝转换，仍保留原文件读取。

单次转换最多处理 2 GiB 输入、5 分钟，最多两个执行槽、32 个处理中记录。Office、RAW 和 PDF 栅格化需要 Bubblewrap；缺少组件、具体格式不支持或处理失败均返回稳定状态，不改变原文件。

固定交付保存文件集合与版本；持续共享跟随目录并排除 review_pending 文件。链接秘密加密持久化，提取码仅存哈希；创建不把分享 URL 当成普通文件引用。撤销阻止新的访问，已传出的字节不能收回。

Eikona 使用现有同源 bearer 上传会话；Scaena 使用其 upload-sessions 与 video-tasks 合同，binding_id 和 model 不同时发送，费用上限、人物类型与确认字段显式传递。不同上传通道不猜测、不自动扩权。服务只保存提示词摘要和领域任务引用，不保存原始提示词或 Provider payload。

提交与回存分别幂等。提交结果不明时查询确定的原任务，不能自动重生成。回存按原产物引用下载、加密暂存、校验并上传到受管空间；失败最多进行三次后台传输尝试，之后由用户恢复。回存结果标记待审核，领域审核真相不迁移到文件服务。

## 兼容和验证

新增表及可选字段采用 GORM 扩展迁移。旧 CLI/HTTP 字段、content 路径、版本引用、token 类型和恢复路径保持存在。回滚旧二进制保留新增表、原始对象、实例身份与密钥；已撤销凭据在旧代码中仍保持 revoked。

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
```

专项测试包含 browser_media_test、media_preview_test、key_archive_test、creative_test 和 scaena_creative_test。Eikona／Scaena 测试为本机 HTTP 模拟服务，未执行付费生成。专业格式的特定软件版本与真实样本仍需运行环境验收，不把组件可发现等同于文件成功转换。

## 登录 Key 目录

`GET /api/v1/key-directory` 返回 Key ID、用途名称、用户 ID/名称、状态、创建/到期时间和 `current` 当前会话标记。普通身份及治理模式仅返回本人凭据；非治理管理员保留既有全局范围。Agent 与服务身份不可调用。明文和 token hash 不出现在响应中。

`POST /api/v1/key-directory/{id}/label` 接受 `name`，仅修改标签（1–80 字，非归档），不改变用户身份、team、权限或凭据。客户端展示 Key → 用户 → team → 网盘的关系。修改 team/网盘绑定仍调用既有治理成员和空间权限接口，不能通过标签转移 Key。


## 分享接收与设备绑定（显式启用）

`drivebridge-server serve --share-receivers` 显式开启挑战证明、设备登记与额度新意图；默认关闭。实现复用原分享/媒体读取与 GORM，不新增传输引擎。设计真源：[change design](../../openspec/changes/drivebridge-share-receiver-v1/design.md)。

- `GET /api/v1/shared/{id}/challenge`、`POST /api/v1/shared/{id}/proof-open`：接收方以链接秘密对单次 nonce 出 HMAC 证明完成打开；秘密不出现在请求体、Cookie 或日志；会话为服务端存哈希的不透明令牌。旧 `POST /api/v1/shared/{id}/open`（grant 请求体）继续可用。
- `POST /api/v1/shared/{id}/devices`、`POST /api/v1/shared/{id}/device-open`：`audience=link` 分享在完整打开（含提取码）后可注册 Ed25519 设备公钥，之后凭设备签名免秘密、免提取码复访；每分享最多 20 设备；Agent 不能注册或使用设备绑定。
- `GET /api/v1/shares/{id}/receivers`、`DELETE /api/v1/shares/{id}/receivers/{device}`：owner 查看假名设备列表并单设备撤销；分享撤销或到期使全部绑定失效。
- 创建请求增可选 `max_opens`、`max_downloads`、`single_use`（等价 1/1）：打开按会话颁发计数，下载按（会话、文件、固定版本）首次建流计数、Range 续传单计；耗尽对未持凭据者与已撤销分享不可区分，不自动删除分享记录。
- 接收类别（anonymous/device/user）由服务端从颁发路径推导，供固定交付回执把观测升级为 `receiver_verified`；不自动产生 `receiver_confirmed`。不采纳端到端加密与 P2P 传输，理由见 change design 对照表。

浏览器先取同源 challenge 再提交 HMAC，只有服务明确返回 `unsupported_action` 才兼容旧 open；服务关闭、证明失败、额度耗尽或撤销均不降级发送链接秘密。设备私钥以不可导出的 WebCrypto `CryptoKey` 保存于 IndexedDB，边界是同一浏览器 profile/origin/share 的设备假名，不是登录账户；只应在可信个人设备明确登记。无 Ed25519/存储能力时保持完整链接访问，不冒称已记住设备。

兼容边界：所有新分享持久标记 `session_required`，即使未启用新特性，也须通过既有 `/open` 获得 opaque HttpOnly 会话。只有真正旧的无额度分享保留原 grant-cookie 过渡；新建或受限分享不能用原secret直接当cookie绕过提取码/打开额度。关闭 `--share-receivers` 不是回滚会话迁移，既有 opaque 会话与额度继续有效。旧二进制不能作为保持这些安全保证的回退手段。

下载额度在首次成功读取并准备发送正文后原子扣除；非法 Range、HEAD、首次存储读取失败均不扣。有效空文件须经EOF验证后计数。nonce单次、五分钟内有效且进程重启失效；设备会话不能再登记设备。

本地证据：`20261008T170226Z-ef16fd2e` 分享HTTP/SDK、安全与重启测试；`20261008T171214Z-b5bc3c7e` 1000次打开/20设备规模门；`20261008T174345Z-83114a09` 三条真实浏览器分享链覆盖proof、不可导出密钥IDB回读、免码复访、owner撤销、额度耗尽和不支持Ed25519的降级。位于项目 `temp/integration-test-runs/`，未访问真实分享/账号或部署。