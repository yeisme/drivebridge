## Context

`crypt cipher.AEAD`（实例密钥 AES-GCM）已加密连接凭据；上传管线=分片暂存→SHA-256→S3 写入→复读验证→事务发布版本；预览=2 MiB 内文本/20 MiB 内媒体流；私有下载授权=5 分钟 S3 presigned URL；`repairParts`/恢复流程复用暂存片。Scaena 参考交接经 socket 获取直链（仅 OneDrive 空间）。

## Decisions

- **密钥层级**：实例 KEK=现有 32B instance.key（HKDF-SHA256 派生 `drivebridge-kek-v1`，不复用凭据加密 AEAD 实例避免密钥用途混用）；每加密空间生成 32B DEK，以 KEK 包裹（AES-GCM，nonce 随机）存 `Space.WrappedDEK`（新列）；解密仅在上传/下载/预览路径内存中发生，DEK 不落盘明文、不进日志/回执。
- **密文格式**：`magic "DBE1" | key_version u8 | nonce 12B | AEAD(payload)`；对象级（整对象流式 AEAD：分块 GCM，chunk 64 KiB，最后块带总长 AAD）——保持大文件流式可解、随机读不必（预览走顺序流）。元数据 `Version.SHA256` 仍为**明文**摘要（校验/去重/manifest/恢复不变，与 backup-restore-v2 对齐）；新增 `encrypted bool` 标记于 Space。
- **路径语义**：上传=读明文→算摘要→加密写暂存→S3 写密文→**复读解密后摘要验证**；下载/预览=读密文→流式解密→明文摘要终验。暂存片存密文（恢复管线复用，不落明文中转）。
- **直链收窄**：加密空间 `POST /files/{id}/access` 不再返回 S3 presigned URL，改为短时服务端解密流端点（同鉴权/CSRF 面，有效期同现值）——持有直链者访问不到密文以外信息，且密文本身无 KEK 不可用。OneDrive 连接器导入/回存对加密空间走受控传输（已是服务端中转）；Scaena 参考交接（依赖 OneDrive 直链）**仅支持非加密空间**，加密空间 prepare/交接明确拒绝并说明（合同场景固化）。
- **opt-in 与不可变**：`space-create --encrypt`（网页/API 同参）；已建空间不可切换加密状态（避免半加密空间）；回收站/版本/配额语义不变。
- **轮换**：KEK 轮换=新 KEK+遍历重包裹 DEK（在线事务，秒级，不重写对象）；DEK 轮换=新版本起用新 DEK，旧版本保留旧包裹（版本不可变对齐）。轮换命令 operator-only（CLI），doctor 可报 key_version 分布。
- **备份**：对象密文天然安全；`backup --encrypt`（见 backup-restore-v2）加密元数据 sqlite 导出（KEK 派生流复用）；**警告写明**：instance.key 丢失=加密空间永久不可解，密钥托管纪律比明文部署更严格（deployment.md 密钥节增强）。
- **零知识取舍记录**：零知识（客户端加密）将破坏明文摘要/预览/服务端去重与 Scaena 交接路径，明确延期；若未来需要，按"客户端持有 DEK 的可选模式"另立 change，不与信封混做。

## Interfaces

`internal/service/crypto_content.go`（信封流式加解密）、`model.go`（Space.Encrypted/WrappedDEK）、`files.go`（上传/下载/预览接线）、`access.go`（直链收窄+解密流端点）、`storage.go`（验证复读改解密后验证）、`main.go`（space-create --encrypt、key-rotate 命令）。

## Verification / Rollback

实现波验证：加密空间全链（上传→复读验证→下载/预览明文一致、分片中断恢复、密钥丢失拒绝、KEK 轮换、直链拒绝、Scaena 交接拒绝、备份含加密空间）+ 明文空间零回归。回滚：加密是 opt-in 新空间属性；不创建加密空间即旧行为，已建加密空间回滚需导出明文（提供 `space-export --decrypt` 逃生口，实现波一并交付）。

## 去向

实现波：信封层+opt-in+直链收窄+轮换+逃生口；backup --encrypt 排本 change 之后。
