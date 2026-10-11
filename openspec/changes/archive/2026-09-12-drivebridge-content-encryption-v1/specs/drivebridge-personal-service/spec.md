## ADDED Requirements

### Requirement: Content envelope encryption
加密空间 SHALL 以实例密钥包裹的 per-space 密钥做服务端信封加密：对象落盘为密文、明文摘要保留在元数据、解密仅发生在服务端授权路径；加密状态创建后不可变。

#### Scenario: Encrypted space roundtrip
- **WHEN** 向 opt-in 加密空间上传并随后下载或预览
- **THEN** 存储侧字节为密文，返回内容与明文摘要一致，密钥材料不进入输出、日志或回执

#### Scenario: Instance key lost
- **WHEN** 加密空间的实例密钥缺失（恢复场景不满足三件一致）
- **THEN** 拒绝服务并明确指向密钥备份，不得以重新生成密钥掩盖（与现有恢复纪律一致）

#### Scenario: Direct-link narrowed for encrypted spaces
- **WHEN** 对加密空间请求私有下载授权或参考交接
- **THEN** 不返回对象存储直链；下载经短时服务端解密流，OneDrive 参考交接明确拒绝并说明原因

#### Scenario: Key rotation without object rewrite
- **WHEN** 操作者轮换实例密钥
- **THEN** 仅重新包裹空间密钥，不重写对象字节，新旧版本可读性保持
