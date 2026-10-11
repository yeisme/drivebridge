## Why

当前文件对象明文落 S3/OneDrive（传输 TLS、连接凭据 AES-GCM、元数据本地）。个人自部署场景下，存储桶/网盘账号泄露即文件泄露；服务端信封加密把"拿到桶凭据≠拿到文件内容"，同时保留预览、校验、去重能力。用户已确认形态：服务端信封（实例 KEK 包裹 per-space DEK），零知识客户端加密延期。

## What Changes（设计，实现归后续波）

- per-space 内容信封：随机 DEK 由实例密钥（KEK，现有 instance.key 派生）包裹存储；对象密文=header(version+nonce)+AES-GCM 密文；元数据保留**明文 SHA-256**（校验/去重/清单不变）。
- 新空间 opt-in `--encrypt`；存量空间不可变不回溯；导入/回存/预览路径服务端透明解密；备份天然密文安全 + 元数据备份导出加密选项。
- 密钥层级与轮换设计：KEK 轮换=重包裹 DEK（不重写对象）；DEK 轮换=仅新版本生效。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增 Content envelope encryption 需求。

## Impact

上传/下载/预览/恢复路径增加透明加解密层；S3 直链（presigned URL）对加密空间不再直接可用——下载必须经服务端解密流（私有下载授权面调整，Access.URL 语义收窄为明文空间）；OneDrive 连接器同理。对象 key 布局不变。SDK/网页无感（字节流仍为明文）。
