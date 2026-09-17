# 实现注记

- Probe 复用 `Generic.remote` 的 `fs.NewFs(remote:root)` 语义；根层 `List`，样本 ≤5，全量计数如实（探测即列举整个根，文档注明大远端耗时）。`--kind onedrive` 的探测结果沿用 onedrive 能力真值，generic 恒 transfer-only。
- `EnsureWebDAVRemote`：vendor=other；密码经 `fs/config/obscure` 后 `SetValue`；已存在同名 remote 时仅当 type/url/user **及密码**一致才复用（obscure 带 nonce，比较走 `Reveal`），密码轮换显式 `remote_exists`，不静默复用旧凭据。
- `EnsureAliasRemote`：local kind 的 `--remote-path` 必须绝对路径（CLI 只做 `IsAbs`，目录 `Clean` 后入库、不做符号链接解析；存在性由 probe 列举保证）。
- `storage add` 旗标映射：backend 恒 `generic`；`--remote-path` 对 gdrive/webdav/smb/sftp/alias/openlist 可空（= 远端根），s3 必填，local 是绝对目录；`spaceAddCommand(c,"storage-add")` + RunE 开头设置 `c.command` 保证成功/失败信封 command 均为 `drivebridge.storage-add`；`--max-bytes` 缺省 1 GiB 仅在该命令生效。
- 失败不留残段：local-root/max-bytes 预检先于任何 remote 创建；本次新建的 alias/webdav remote 在 probe 或委托失败时经 `DeleteRemote` 回滚。
- 凭据文件纪律对齐 scaenaTokenReader：Lstat+普通文件+0077 拒绝+8192B 上限+TrimSpace；`credential_file_not_private` 进入 recovery registry 并与 docs 表同步。
- 错误码进 `internal/recovery` registry（HintEN/DocZH），docs 表同步测试强制更新；配置落盘失败映射 `config_write_failed`（rclone `SaveConfig` 只打日志，改用 `Data().Save()` 取错误）。
- 机器模式单信封：探测摘要在人读模式前置打印，机器模式只保留最终信封。
