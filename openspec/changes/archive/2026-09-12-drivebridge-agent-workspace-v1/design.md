## Context

agent 需要组织文件而非平铺根层；终端/脚本用户需要不经 MCP 的直连传输；MCP 客户端注册是文档里的长命令行。cobra/viper 配置面（cli-config-v1）与 generic provider（generic-backends-v1）已就绪。

## Decisions

- **路径模型**：目录=工作区相对路径（`remoteDir` 校验：空/`.`=根、`filepath.IsLocal`+clean、拒绝绝对与 `..`）；OneDrive 用 Graph path 寻址（`root:/<dir>:` 解析 folder item id，children 过滤 parent==folderID——沿空间根钉定不变）；generic=rclone `f.List(ctx, dir)`/`f.Mkdir(ctx, dir)`（rclone Mkdir 幂等合同即设计语义）。
- **mkdir 幂等**：OneDrive 逐段 `POST children {"folder":{}, "@name.conflictBehavior":"fail"}`，409 时列表确认同名 folder 则下探、同名文件则 `target_conflict`；已存在完整路径直接成功。
- **upload 目录**：`op.Target = dir + "/" + <id>-<name>`；既有目标扫描限定在 `filepath.Dir(Target)`；指纹校验改为取 Target basename 去 id 前缀（旧 `[33:]` 切片对带目录目标错位——实现中实踩）。OneDrive 上传经 rclone fs 层 Put 路径名天然支持；返回 item id 复用 IDer→Stat 链。
- **CLI 门语义**：`--enabled` 只约束 MCP 写入口（既有合同）；CLI 子命令在用户本人终端执行=用户动作，openService(true)。设计明示这一区分，防未来误读为"CLI 绕过安全门"。
- **CLI 幂等键**：缺省 `cli-<sha256(space|file|name)[:8]>`（下载 `ref|out`）——同一命令重试命中原 operation（enqueue 幂等路径），不产生重复文件。
- **install**：claude-code shell 出官方 CLI（无则打印等价命令）；codex 追加 TOML（已注册则如实报告）；generic 打印 stdio 片段。绝不写 token（注册项本身无凭据）。
- **能力面**：capabilities 的 tools 已含新工具；目录能力对所有 backend kind 开放（onedrive+generic）。

## Interfaces

`internal/bridge/model.go`（Provider.List(dir)/Mkdir、Service.List/Mkdir、remoteDir）、`onedrive.go`（folder/childFolder/graphPost/Mkdir）、`generic.go`、`transfer.go`（目录目标+指纹校验）、`cmd/drivebridge/commands.go`（六个传输命令+install）。

## Verification / Rollback

alias fixture 全链（mkdir 幂等→upload 进目录→按路径 list→download 回读→逃逸/root mkdir 拒绝）+ OneDrive fake Graph path 用例 + testscript（config 驱动全链+CLI 重试幂等+install generic 无 token）+ race/vet/CGO-free + 真实 stdio 冒烟。回滚：不传 path/dir 即根层旧行为；CLI/install 为新增子命令零影响。

## 去向

本 change 完成实现；soft-delete（trash/restore）与 discovery（search/changes 游标/内容寻址）见并行设计 change。
