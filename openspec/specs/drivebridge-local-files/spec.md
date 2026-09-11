# drivebridge-local-files Specification

## Purpose
TBD - created by archiving change drivebridge-local-mcp-v1. Update Purpose after archive.
## Requirements
### Requirement: Stable identity and private transport
系统 SHALL 使用稳定文件引用并隔离空间、身份、目录和文件版本，临时下载 URL 只交给已配置的本机 owner adapter。

#### Scenario: Allowed reference
- **WHEN** 原任务身份有权消费指定空间的固定版本文件
- **THEN** 系统返回稳定引用与校验摘要，在实际提交前通过私有通道获取下载授权

#### Scenario: Wrong identity or replaced file
- **WHEN** 访问者身份、空间或文件版本不匹配
- **THEN** 系统拒绝交接，不返回下载 URL

### Requirement: Recovery does not repeat generation
系统 SHALL 使用原操作/原任务进行恢复，禁止因网络错误或回存失败自动发起新的生成。

#### Scenario: Lost upload acknowledgement
- **WHEN** 上传已完成但客户端没有取得回执
- **THEN** 恢复核对原操作的远端文件摘要并返回同一文件，不重复创建资产

#### Scenario: Output save fails
- **WHEN** 成功生成任务的成品回存失败
- **THEN** 只恢复文件传输，保留原领域任务与审核状态

### Requirement: Explicit activation and honest readiness
系统 SHALL 默认关闭新增写入口，并区分本地 fixture 验证与真实账号、Gateway、Provider 区域验收。

#### Scenario: Secure handoff absent
- **WHEN** 宿主没有可用的私有交接通道
- **THEN** 系统明确拒绝参考交接，不把 URL 写入普通 MCP 输出，也不自动配置 TOS

### Requirement: Preflight readiness reporting
doctor SHALL 以只读方式报告激活所需预检项，按名区分 pass/fail/info；不得获取写锁、发起真实账号探测，或在输出中包含凭据、临时 URL 或原始第三方错误。

#### Scenario: All prerequisites met
- **WHEN** 本机预检全部通过（私有 state、rclone 远端、空间、可写 local root、ffprobe，及可选的 handoff socket 与 Scaena 端点）
- **THEN** 命令汇总 ready 并以 0 退出

#### Scenario: Missing or broken prerequisite
- **WHEN** 任一预检项缺失或损坏（无空间、local root 不可写、残留 socket、凭据文件权限过宽等）
- **THEN** 按名报告失败项并以非 0 退出，明细不泄露 token、URL 或原始错误正文

#### Scenario: Remote mode preflight
- **WHEN** 以 `--remote-url` 运行 doctor
- **THEN** 校验服务地址形状、token 文件权限与 local root，并探测 `/healthz` 可达性；可达不代表授权有效，授权状态记为 not probed

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

