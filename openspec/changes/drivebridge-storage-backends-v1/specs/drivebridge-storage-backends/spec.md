## ADDED Requirements

### Requirement: 按后端的能力矩阵如实报告

系统 SHALL 按 `backend_capabilities` 对每个后端类别如实上报 transfer、server_side_copy、native_digest、direct_url、reference_handoff；未实现或未知的键 SHALL 视为 false 并在尝试时返回既有固定拒绝码（`access_unsupported` / `handoff_not_available` / `unsupported`）。

#### Scenario: generic 空间请求直链
- **WHEN** OpenList/NAS/S3 等 generic 空间请求临时直链或参考交接
- **THEN** 返回固定拒绝码，不降级为其他传输伪装成功

#### Scenario: 晋升后端能力可见
- **WHEN** named 后端（如 `gdrive`）注册了 server_side_copy 与 native_digest
- **THEN** `capabilities` 上报该键，未注册的键不出现或为 false

### Requirement: 版本钉住按后端声明且漂移必须可见

系统 SHALL 按后端声明的键钉住版本（OneDrive=item+ETag；generic=大小+mtime；gdrive 晋升=file_id+native_digest+mtime；WebDAV=ETag 否则大小+mtime）。钉住要素变化时 SHALL 报 `file_version_changed`，SHALL NOT 静默复用旧字节或静默跟随 latest。

#### Scenario: OpenList 网盘 mtime 抖动
- **WHEN** 国内网盘经 OpenList 修改了文件元数据或内容
- **THEN** 消费前 stat 失败为 `file_version_changed`，重新 list 获取新引用

#### Scenario: gdrive 内容被替换
- **WHEN** 同一 file_id 的 MD5 变化
- **THEN** 旧引用拒绝消费，不把新字节冒充旧版本

### Requirement: 原生哈希只做变更检测

provider 原生哈希（MD5/ETag/SHA1）SHALL 只用于变更检测、展示与 stat；blob 内容身份 SHALL 恒为 DriveBridge 计算的明文 SHA-256。系统 SHALL NOT 用原生哈希授权读取、秒传跨范围匹配或替代内容校验。

#### Scenario: 秒传判定
- **WHEN** 本机文件 SHA-256 与空间索引命中
- **THEN** 秒传决策与现有 ReuseUpload 语义一致；provider MD5 不扩大命中范围

#### Scenario: 下载校验
- **WHEN** 下载完成
- **THEN** 以 DriveBridge 记录的 SHA-256 校验，原生哈希不一致按变更处理

### Requirement: 第三方聚合器的信任边界

接入 OpenList 等聚合器 SHALL 以自托管为前提：钉住已验收版本、使用专用受限账号、凭据保存在 0600 文件或 rclone 配置；REST 直链（若未来启用）SHALL 沿用 OneDrive URL 纪律——临时 URL 只在 owner 进程提交时刻获取，不进 MCP 输出、日志、回执或持久化，默认关闭。

#### Scenario: OpenList 凭据
- **WHEN** 空间经由 OpenList WebDAV 配置
- **THEN** 账号密码只在 rclone 配置/0600 文件，MCP 与日志不出现

#### Scenario: 直链默认关闭
- **WHEN** 当前版本未启用 OpenList 直链 provider
- **THEN** `backend_capabilities` 不宣称 direct_url，请求按固定拒绝码处理

### Requirement: 平台卷与个人空间不混用

实例拥有的平台 blob 卷（`platform-connect`）与用户拥有的个人空间后端 SHALL 保持分离：平台卷承载 CAS 去重与可选信封加密，个人空间后端不做实例级去重；本机 generic 空间 SHALL 如实报告为远端明文（无 Server 信封加密）。

#### Scenario: MinIO 双角色
- **WHEN** 同一 MinIO 同时作为平台卷与某用户 generic 空间
- **THEN** 平台卷走实例命名空间与隔离探针；generic 空间独立，互不借权限

#### Scenario: 诚实加密声明
- **WHEN** 用户询问 OpenList/NAS 空间是否加密
- **THEN** 如实报告仅远端自身保护；需要信封加密须使用 Server 托管空间

### Requirement: 增量接入与回滚

named 后端与能力晋升 SHALL 是加法：不修改存量 generic 空间语义、不改旧 URI 与错误码；回滚 = 移除能力键或删除空间，保留远端数据与既有引用。

#### Scenario: gdrive 晋升回退
- **WHEN** 操作者关闭 named gdrive 晋升
- **THEN** 相关空间回退 generic 行为；已登记引用按当前钉住键继续解析或诚实报变化
