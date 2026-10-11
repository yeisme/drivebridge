# 生产部署 runbook

面向把 DriveBridge Server 从本地候选推进到真实自部署的操作者清单。首版为单实例；真实 OIDC/S3/微软注册、公网域名与 TLS 由操作者独立完成，本 runbook 提供步骤、预检与回滚，不替代运行时验收。

## 0. 前置条件

- 域名与 TLS 终结（反代或直接 TLS）；程序默认只监听回环地址，公网暴露由操作者显式配置
- 标准 OIDC provider（如 Ory Hydra / Ory Network；Kratos 登录 UI 本身不等于 issuer）
- 平台 S3：HTTPS 公网端点（生产连接默认拒绝重定向、私网/回环数据端点）
- 网页构建产物：`cli/drivebridge/web` 执行 `pnpm build` 得到 `dist/`
- 私有 state 目录（权限 0700，绝对路径）

## 1. OIDC 应用注册

- 在你的 issuer 上注册应用；精确回调地址 `<origin>/auth/callback`
- 始终使用 PKCE；公共客户端可不提供 secret，confidential 客户端 secret 存 0600 文件（`--client-secret-file`）
- 记录**具体 issuer**（经 discovery 验证；不支持 `common` 等多租户入口）、client-id
- 每部署一个 issuer；用户按 `(issuer, subject)` 关联，不按邮箱合并

## 2. OneDrive 连接器注册（可选）

- 微软应用注册；回调 `<origin>/auth/onedrive/callback`
- 委托权限 `Files.ReadWrite` + `offline_access`
- 启用参数 `--onedrive-client-id`（可选 `--onedrive-secret-file`）

## 3. state 与平台存储

```bash
./drivebridge-server platform-connect --state /absolute/private/drivebridge-state < /absolute/private/platform-s3.json
```

输入字段 `endpoint/region/bucket/prefix/access_key/secret_key`（可选 `session_token`、`virtual_hosted`）。凭据文件由你的凭据管理流程提供，不进仓库；首次连接会探测可达性，实际写权限在上传时验证。首版不允许原地重绑平台存储。

拒绝 path-style 寻址的 S3 兼容提供方（如腾讯云 COS）须声明 `"virtual_hosted": true`，endpoint 写区域主机（例：`https://cos.ap-guangzhou.myqcloud.com` + `region: ap-guangzhou`），桶以子域寻址；不声明时保持 path-style，已封存连接零迁移。COS 拒绝 path-style 时报 `PathStyleDomainForbidden`，且 HEAD 类请求无错误体会退化为裸 `Forbidden`——先核对寻址模式再排查凭据。

## 4. 首次启动序列

0. **写部署配置（推荐，一次即可）**：
   ```bash
   ./drivebridge-server config init          # 写 ~/.config/drivebridge-server/config.yaml
   $EDITOR ~/.config/drivebridge-server/config.yaml
   ./drivebridge-server config show          # 每键来源：flag/env/file/default
   ```
   发现序：`--config` > `DRIVEBRIDGE_SERVER_CONFIG` > `./drivebridge-server.*` > 用户配置目录 > `/etc/drivebridge-server/config.*`；优先级 显式旗标 > `DRIVEBRIDGE_SERVER_*` env > 文件 > 默认；**未知键/带路径 origin/非正整数 max_bytes 直接报错**。`--enabled/--development` 与 secret 值只能显式命令行，文件只承载路径、URL 与限额。
1. **不带 `--enabled` 启动**（业务入口关闭，只听回环；state/listen/web-dir 走配置）：
   ```bash
   ./drivebridge-server serve                # 或等价：serve --state ... --listen 127.0.0.1:8787 --web-dir .../dist
   ```
   确认 `/healthz` 与静态页可达。
2. **部署预检（新命令）**：
   ```bash
   ./drivebridge-server doctor --state ... --web-dir .../dist --issuer https://your-oidc-issuer.example.com
   ```
   全部 pass 再继续；`state_consistency` 失败按恢复指引处理，不要删除重建。issuer 探测只验证 discovery 文档与 issuer 字段一致，不代表账号级验收。
3. **带 `--enabled` 启动**（origin/issuer/OIDC 参数已在配置文件，旗标等价路径仍可用）→ 浏览器登录、创建空间、上传一个小文件完成首次冒烟。

## 5. 备份与恢复演练

```bash
./drivebridge-server backup --state ... --output /absolute/new/backup-directory
```

备份目录三件：`metadata.sqlite`、`instance.id`、`manifest.json`（对象清单：版本→对象 key→明文摘要，全部 0600）。

- `instance.key` 不在备份内，必须用独立秘密渠道保管（单人场景：密码管理器或离线介质；不用双人门）
- 恢复：新 0700 state 目录放入 `metadata.sqlite`、`instance.id` 与**原** `instance.key` → `doctor` 确认 `state_consistency` pass → 启动
- 缺失上传暂存片的操作会标为需原文件重传；已完成 S3 文件无需重传
- S3 字节备份/版本策略由存储操作者管理；清单提供校验依据

### 5.1 只读校验

```bash
./drivebridge-server backup --state ... --verify /absolute/backup-directory [--verify-sample N]
./drivebridge-server doctor --state ... --backup /absolute/backup-directory
```

- `--verify` 三段：清单↔元数据交叉、抽样对象存在性（默认 min(50, 全量)）、三件一致性；固定失败码 `manifest_mismatch` / `object_missing` / `sample_unverifiable`
- `doctor --backup` 纯文件级只读校验（不需要连接凭据），供恢复演练后复核
- 校验不过=不可信备份：整体判定不可恢复，不存在"部分通过"

### 5.2 加密备份包（可选）

```bash
./drivebridge-server backup --state ... --output /absolute/new/dir --encrypt   # 写 backup-bundle.enc
./drivebridge-server backup --state ... --decrypt /absolute/dir/backup-bundle.enc --output /absolute/extract-dir
```

- 加密用实例密钥独立派生流（AES-GCM 包 tarball）；`instance.key` 丢失=备份不可解
- 解出的目录用 `doctor --backup` 复核后再进入恢复

### 5.3 恢复演练清单（脚本化冒烟见 `internal/service/backup_test.go` TestRestoreDrillScriptedSmoke）

1. [ ] 备份：`backup --state ... --output <dir>`（或 `--encrypt`）
2. [ ] 校验备份：`backup --verify <dir>` 全绿（加密包先 `--decrypt`）
3. [ ] 从托管渠道取回 `instance.key`（演练必须包含这一步）
4. [ ] 隔离 state 目录放入三件+原 `instance.key`（0700/0600）
5. [ ] `doctor --state <新state> --backup <dir>` 全绿
6. [ ] 不带 `--enabled` 起 serve 冒烟（`/healthz` 可达）
7. [ ] 清理演练目录

## 6. 部署即租户与多实例分片

- **租户=部署实例**：`instance.id` 是租户唯一标识（远程引用 `drivebridge://<instance-id>/file/...` 已携带）；跨实例访问仅经目标实例签发的限定 Agent token，实例间不提供委托、联邦或共享；引用实例身份不匹配时按 `cross_instance_rejected` 拒绝
- **租户 profile**：`serve --tenant-name/--tenant-origin/--tenant-contact` 启动时写入元数据，登录用户经 `GET /api/v1/tenant` 只读；在线面不可改，修改=停机改启动参数（不带参数重启保留既有 profile）
- **多租户扩展**：按实例分片部署——每租户独立 state 目录、独立 origin/TLS、独立 issuer（或同 issuer 不同 client）；不做单实例内租户化，不承诺跨实例数据迁移

### 共享桶多部署（共享存储凭据）

分片部署可以共享同一个对象存储桶：平台存储对象键恒为 `<instance-id>/<user>/<space>/<version>`，部署间命名空间天然不重叠，`prefix` 可再叠加一层租户分组（如 `tenants/<tenant>/`）。

- **信任边界（诚实语义）**：共享同一存储凭据的部署在存储层**没有字节级隔离**——同一凭据技术上可触达桶内任意对象。两条缓解路径：
  1. **前缀作用域子凭据（推荐）**：在腾讯云 CAM 为主机账号签发子凭据，策略资源限制到本部署命名空间，例如 `qcs::cos:ap-guangzhou:uid/<appid>:drivebridge-1384001538/tenants/<tenant>/<instance-id>/*`，动作收窄到 `cos:PutObject/GetObject/DeleteObject/HeadObject/ListObjects`；子凭据经 `platform-connect` stdin 进入，root 凭据只留在云控制台。平台不代签发云子凭据。
  2. **空间信封加密（兜底）**：加密空间的密文只有持有实例 KEK 的部署可解（见 §7）；即便凭据超域，其他部署读不出明文。
- **隔离验证**：`platform-connect --verify-isolation` 在保留探针命名空间（`_isolation-probe/` 与前缀下兄弟形路径）尝试写入并期待被拒：两处都被拒报告 `isolated`（子凭据作用域正确）；任何一处意外成功立即删除该探针对象并报告 `over_scoped`（root 凭据即此结果）。探针不触碰真实租户命名空间；已配置过的部署可重复执行同一命令做复核（连接失败 `platform_storage_already_configured` 前会先输出探针结论）。
- **同桶多部署配方**：每部署独立 state；`platform-connect` 输入同 bucket/endpoint，`prefix` 与子凭据按部署区分；`--verify-isolation` 确认 `isolated` 后再 `serve`。

### 新租户四步与 key 用户（无 OAuth）

新租户部署的标准序列（key-only 模式不需要 OIDC/issuer）：

```bash
# ① 部署配置（state/listen/origin 等）
./drivebridge-server config init && $EDITOR ~/.config/drivebridge-server/config.yaml
# ② 平台存储（共享桶前缀作用域 key，见上节）
./drivebridge-server platform-connect --verify-isolation < platform-s3.json
# ③ 批量签发用户 key（stdin JSON；不带 key 由系统生成；days=0 永不过期）
# 推荐直接落 0600 分发文件（明文 key 只出现一次，stdout/文件二选一，别靠手抄）
echo '[{"name":"user-a"},{"name":"user-b","days":90},{"name":"ops","admin":true}]' | ./drivebridge-server users import --out dist.csv
# ④ 启动（未配置 issuer 即 key-only；浏览器登录如实报 oidc_not_configured）
./drivebridge-server serve --enabled
# ⑤ 启动后核对（远程：healthz+身份+空间单投影；本地：state 快照+配额+key 计数）
./drivebridge-server status --url https://<origin> --token-file admin.key
```

- **用户 key 即身份**：自生成为 `dbu_`+96hex；也可**导入外部预生成 key**（任意前缀，32–128 可打印 ASCII，含空白的表项会被拒绝——熵下限防爆破）。Bearer `Authorization: Bearer <key>` 直接调 API；库内只存 SHA-256，无自助注册、无 OAuth。重复或与 Agent 令牌哈希冲突整批拒绝零落库。
- **密钥轮换**：`users rotate --id <key-id> [--days N]` 同用户原子换发（铸新+吊销旧，明文一次输出；`--days 0` 沿用剩余有效期）——泄露处置一步完成，旧密钥及其换发会话即时失效。
- **管理面**：`users list`（无哈希无明文）/`users revoke --id`（即时失效）；HTTP `POST /api/v1/users` 仅 admin 用户密钥可用（普通 key/Agent/会话一律 forbidden）。
- **Agent 令牌直发**：`agents create --space <id> --name <n> --scopes read,write [--days N] [--out token.file]`——operator 在本机状态目录直接签发（owner 从空间行推导），key-only 部署无需 session+CSRF 的 HTTP 往返；`agents list` 只看生命周期元数据。明文令牌与用户 key 同纪律：只出现一次，`--out` 落 0600。
- **配额口径**：`/api/v1/spaces` 与 MCP `spaces` 每行带 `max_bytes/used_bytes/reserved_bytes`，余量 = `max - used - reserved`（reserved 是在途操作占用）；`drivebridge-server status` 本地模式直接给三值合计。
- **填表批量开通（推荐）**：`users import --csv` 收表（表头 `name,days,admin,key` 或按位无表头；单列即纯名单；表头模式下未命名列当备注忽略），`--out dist.csv` 写 0600 分发文件（name,user_id,key,admin,expires_at）：

  ```bash
  DRIVEBRIDGE_STATE=<state> python3 scripts/issue_users.py scripts/users-template.csv -o dist.csv
  # 输出 dist.csv：name,drivebridge_key,expires_at——用户拿 drivebridge key
  # 走文件/MCP/网页，一表一次发完。
  ```
- **泄露处置**：签发终端只输出一次；本人可经网页再次取回加密保存的 Key。泄露后应立即轮转或使用 `users revoke` 吊销，见[登录 Key 管理](key-management.md)。
- **浏览器登录（key-only 部署）**：`POST /auth/key` 用密钥换发与 OIDC 同构的 12h cookie 会话（浏览器不存密钥）；`GET /auth/mode` 如实报告登录方式。admin 密钥换发的会话继承 admin（`GET /api/v1/users`、`POST /api/v1/users/{id}/revoke`、`POST /api/v1/users` 均可）；**吊销/轮换即级联**——来源密钥失效后其换发会话下一次请求即 401（OIDC 会话不受影响）。

### 自托管激活（Tailscale + key-only + S3-only）

面向个人/受邀试用者规模的首验形态：不接 OIDC、不接 OneDrive、不暴露公网。实例即租户，多租户按 §6 实例分片扩展。

**离线彩排（真实部署前必跑）**：四步序列可用真实二进制离线预演，把配置与序列错误留在本地：

```bash
python3 scripts/check.py -- python3 scripts/selfhosted_rehearsal.py
```

彩排用真实 `drivebridge-server` 二进制、真实网页 `dist` 与回环 S3 fixture（`drivebridge-fixture --s3-listen` 独立暴露）走通 platform-connect → users import → serve --enabled → status 远程核对，再做浏览器等价冒烟（`/auth/mode` 如实报告 key-only、密钥换会话、建空间、上传/下载比对、配额口径）与备份只读校验。`platform-connect --development` 是显式 CLI 开关，只用于此类离线彩排，不进配置文件。明文 key 只落在证据 artifacts 的 0600 `dist.csv`，操作者阅后清理。

**真实部署步骤（需操作者逐项确认输入）**：

1. 目标机准备：长期运行的服务器；`tailscale status` 确认设备在线；创建 0700 state 目录；按目标 OS/arch 以 `CGO_ENABLED=0 go build` 构建二进制与网页 `dist`。
2. 平台存储：按 §3 准备 S3 凭据 JSON（共享桶多部署时按 §6 用前缀作用域子凭据），`platform-connect --verify-isolation < platform-s3.json`（**不带** `--development`；端点必须 HTTPS 公网）。
3. 签发用户：`users import --csv --out dist.csv`（表头 `name,days,admin,key`；至少一枚 admin），分发文件 0600 只出现一次。
4. 启动：`serve --state ... --listen 127.0.0.1:8787 --origin https://<tailscale-hostname> --web-dir .../dist --enabled`。监听保持回环，由 Tailscale（`tailscale serve` 或设备直连端口）暴露给本人设备；`--origin` 必须与浏览器实际 origin 完全一致。
5. 首验门（全部通过才算激活）：本人另一台设备浏览器打开 origin → `/auth/mode` 报告 key-only → 用户 key 登录 → 建空间（`connection_id` 选平台存储）→ 上传/下载一个小文件并比对 → 配额三值可见 → `status --url <origin> --token-file admin.key` 成功 → 停服跑一次 `backup` + `doctor --backup`。
6. 密钥纪律：`instance.key` 立即入独立秘密渠道；泄露处置 = `users revoke` + 重签。

**回滚**：去掉 `--enabled` 或停止进程即关闭业务入口；state、对象与回执全部保留。

## 7. 内容信封加密（opt-in 空间）

- 创建空间时 `encrypted: true`（API/网页同参）启用服务端信封加密：per-space DEK 由实例 KEK（instance.key HKDF 派生）包裹；对象密文格式 `DBE1 | key_version | nonce | 分块 AES-GCM`；元数据保留**明文** SHA-256 与逻辑大小
- 加密状态创建后不可变；存量空间不回溯；上传/下载/预览/导入/回存服务端透明加解密，本地暂存片也是密文
- **直链收窄**：加密空间 `POST /files/{id}/access` 返回 5 分钟短时服务端解密流（`/api/v1/access/<token>`），不再返回 S3 presigned URL；MCP `prepare_reference` 对加密空间拒绝（`encrypted_space_reference_rejected`），交接走解密流
- **密钥纪律**：`instance.key` 丢失=加密空间永久不可解（比明文部署更严格）；`doctor` 的 `content_encryption` 检查报 key_version 分布
- **轮换与逃生口**：`key-rotate` 只重包裹空间 DEK，不重写对象（新旧版本都可读）；`space-export --space <id> --output <dir>` 把加密空间当前版本导出为明文（回收站文件跳过并计数），是回滚前的逃生口

## 8. 反向代理要点

- `--origin` 必须与浏览器实际 origin 完全一致（写请求校验固定 Origin + 会话 CSRF）
- 不用关闭 CSRF 检查来解决跨端口问题；开发用 Vite 代理时设 `--origin` 为浏览器 origin
- 开发回环 HTTP 须显式 `--development`；生产数据端点必须 HTTPS 公网

## 9. 回滚

去掉 `--enabled` 或停止进程即可关闭业务入口；state、对象与回执全部保留。完整回滚：停进程、保留 state 目录与备份。

## 10. doctor 失败对照

| 失败项 | 处置 |
|---|---|
| `state_dir_not_private` | `chmod 0700 <state>` |
| `instance_key`/`instance_id` 缺失（库存在） | 从秘密备份恢复对应文件；**不要**删除数据库重建 |
| `state_consistency: instance_key_missing_restore_required` | 同上——重新生成密钥会使已加密连接凭据不可解 |
| `metadata_db_unreadable` | 检查文件权限/完整性；必要时从备份恢复 |
| `platform_storage_missing` | 执行 `platform-connect` |
| `web_dir_missing_index` | 重新构建 `cli/drivebridge/web` 并指向 `dist/` |
| `issuer_unreachable` / `issuer_mismatch` | 核对 issuer URL；不支持多租户入口 |
| `state_in_use`（serve 时） | 已有实例持有锁；勿双进程共 state |
| `backup_manifest_invalid` | 清单缺失/损坏：重新 `backup` 生成；旧备份（无清单时代）可重新备份一次补齐 |
| `backup_crosscheck: manifest_mismatch:*` | 清单与元数据漂移：以重新备份为准，勿手工修补清单 |
| `content_encryption: encrypted_space_dek_unreadable` | wrapped DEK 损坏：从更早备份恢复 state；无法恢复时对象不可解 |
