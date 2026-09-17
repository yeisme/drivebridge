## ADDED Requirements

### Requirement: 本机下载 CAS

系统 SHALL 在 local_root 内维护内容寻址缓存，并在目标路径或缓存命中且与固定 version 摘要一致时跳过拉取。

#### Scenario: 二次下载
- **WHEN** 同一 version 下载到新文件名且缓存已有该 SHA-256
- **THEN** 不访问远端字节，目标文件摘要一致

#### Scenario: 源 version 已变
- **WHEN** 缓存摘要与 stat 返回的当前 version 不符
- **THEN** 不发布旧字节到目标路径

### Requirement: WebDAV 挂载默认只读回环

`drivebridge mount webdav` SHALL 仅在用户终端启动，默认回环、只读、显式空间列表。Agent token SHALL 不能启动挂载。

#### Scenario: 未授权空间
- **WHEN** WebDAV 客户端请求未授权空间路径
- **THEN** 返回拒绝且不泄露该空间文件名

#### Scenario: 默认 FUSE
- **WHEN** 默认 CGO=0 二进制执行 `drivebridge mount fuse`
- **THEN** 以 `unsupported` 失败并提示 WebDAV

### Requirement: VFS 操作映射

WebDAV 与可选 FUSE SHALL 只翻译根包 vfs-api：Lookup/Readdir/Open/Create/Mkdir/Rename/Unlink/Statfs。setattr、chmod、LOCK SHALL 返回 `unsupported`。Unlink SHALL trash。Open SHALL 冻结 version。部分写 SHALL 在 PUT/release 结束前不发布 version。

#### Scenario: Range GET
- **WHEN** WebDAV 客户端对文件 GET 带 Range
- **THEN** 返回部分字节，远端 version 不变

#### Scenario: 同 state 第二挂载
- **WHEN** 已有 webdav 进程时再次 mount
- **THEN** 失败为 `mount_already_running`

#### Scenario: 非回环
- **WHEN** `--listen 0.0.0.0:19080` 且无 `--lan`
- **THEN** 失败为 `listen_not_allowed`

### Requirement: MCP 只读挂载状态

MCP SHALL 最多提供 `mount_status`。能力资源 SHALL 报告 `mount.start=cli_only`。Agent token SHALL 不能认证 WebDAV。

#### Scenario: tools/list
- **WHEN** Agent 列出工具
- **THEN** 无 mount_start / mount_stop
