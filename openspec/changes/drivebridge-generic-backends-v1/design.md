## Context

`Provider` 接口（model.go）五个方法中 `List/Stat/Upload/Open` 天然映射 rclone `fs.Fs`；OneDrive 特化点只有 mapper 校验（`type=onedrive`+drive_id）与 Graph 直调（`item()`/`Access()`）。rclone `fs.Find(type)` + `fs.ConfigMap` 可构造任意后端实例，token/凭据刷新全部由 rclone 后端自身处理。

## Decisions

- **Space 增字段** `Backend string`（空=`onedrive`，向后兼容）。`space-add` 增 `--backend <kind>`：`generic` 时以 `--rclone-remote <name>` 指向既有 rclone 配置节（复用无缝登录/rclone 配置作为唯一凭据面），`--remote-path` 指定根（如 `nas:media/refs`）；DriveID/RootID 对 generic 无意义，改存规范化根路径。空间身份（含 Backend）创建后不可变，与现行绑定语义一致。
- **generic provider**（新文件 `internal/bridge/generic.go`）：`fs.NewFs(ctx, remotePath)` 构造；List=walk 根下一层、Stat/Object Attributes、Upload=`f.Put`（操作命名沿用 `<operation-id>-<name>`）、Open=`o.Open`。`Access()` 返回 `access_unsupported`。错误分类映射固定码（`backend_unavailable`、`path_not_allowed`…），不透传原始错误。
- **能力矩阵**：capabilities 资源与 `tools/list` 注记 per-space kind：`transfer: list/stat/upload/download/resume`、`direct_url: onedrive only`、`reference_handoff: onedrive only`。`prepare_reference` 对 generic 空间返回 `handoff_not_available`，不降级为下载链接。
- **软链接策略**：上传/同步扫描遇 symlink（含指向 local-root 外）默认拒绝（`symlink_rejected`），防越界逃逸；space 级 `--follow-symlinks` 显式 opt-in 后物化拷贝（内容副本，对象端无 symlink 概念）。opt-in 状态入 Space 记录并在 capabilities 可见。
- **网络存储边界**：SMB/NFS/WebDAV/SFTP/对象存储/SaaS 盘=消费面（generic 后端目标）；`rclone serve`/FUSE mount（把 DriveBridge 空间暴露给系统挂载）明确延期，本设计不建挂载面。
- **测试面**：fake rclone 后端用 `local` 后端 + 临时目录（rclone 自带 local fs 即可离线测全链），不引网络 fixture。

## Interfaces

`internal/bridge/generic.go`（provider）、`model.go`（Space.Backend/FollowSymlinks + 校验）、`mcp.go`（能力矩阵）、`cmd/drivebridge/main.go`（space-add 旗标）。

## Verification / Rollback

实现波验证：local-backend fixture 全链（list/stat/upload/download/resume/软链接三态/能力矩阵）+ race + stdio 冒烟；OneDrive 既有测试零改动。回滚：generic 是新增枚举值，不注册 generic 空间即完全回退。

## 去向

实现波：generic provider + 软链接三态 + 能力矩阵；依赖 seamless-auth-v1 的配置 UX（可并行）。
