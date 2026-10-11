## Why

用户要求复制真实登录 Key、再次复制及轮转，并明确替代原来的标识复制交互。

## What Changes

- 增量添加 UserKey.SealedKey，以实例 AES-GCM 加密，AAD 绑定用户与 Key ID；哈希认证仍保持。
- 新增本人 reveal、rotate 接口和目录能力字段。管理角色不获得他人密钥读取权。
- 轮转原子创建后继并停用旧 Key，只迁移发起操作的当前浏览器会话。
- 旧记录不伪造明文：原 Key 登录时补齐密文，或本人轮转后使用新 Key。

## Capabilities

### New Capabilities
- `key-self-service`: 本人重复取回及轮转。

### Modified Capabilities
- `drivebridge-personal-service`: 用户 Key 加密保存及本人轮转会话语义。

## Impact

fit：身份、GORM 持久化和权限归本服务；网页交互为 split-owner，由 client/drivebridge-client 消费。Agent token 与恢复码维持原合同；旧 CLI/API 签发、认证、吊销和管理员轮转继续可用。无需真实部署、云凭据或外部付费调用。
