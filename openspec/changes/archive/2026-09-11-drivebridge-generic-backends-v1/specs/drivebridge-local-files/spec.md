## ADDED Requirements

### Requirement: Backend kind capability matrix
空间 SHALL 声明后端类别并在能力面如实区分传输能力与 OneDrive 专属能力；非 OneDrive 后端不得暴露临时直链或参考交接。

#### Scenario: Generic rclone backend space
- **WHEN** 空间以 generic 后端（如 SMB/WebDAV/S3/local）配置
- **THEN** list/stat/upload/download 与操作恢复可用，`prepare_reference` 与直链获取返回固定拒绝码，不降级为其他交付方式

#### Scenario: Honest capability reporting
- **WHEN** 能力资源或 tools/list 描述空间
- **THEN** 按 kind 报告 transfer/direct_url/reference_handoff 能力，未配置的后端不虚构支持

### Requirement: Symbolic link policy
传输与同步 SHALL 默认拒绝符号链接以防越界逃逸；仅空间级显式 opt-in 后以物化拷贝跟随。

#### Scenario: Symlink encountered by default
- **WHEN** 上传或同步扫描遇到符号链接（含指向 local root 之外）
- **THEN** 以固定错误码拒绝该项，不跟随、不跳过不报，也不写入对象

#### Scenario: Explicit follow opt-in
- **WHEN** 空间显式启用 follow-symlinks
- **THEN** 链接以内容副本物化上传，对象端不保留链接语义，opt-in 状态在能力面可见
