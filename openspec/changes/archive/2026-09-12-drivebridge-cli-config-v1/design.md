## Context

沿 aigora viper 先例（typed Unmarshal+Validate、文件发现序）与 scaena cobra 先例（薄 main + 集中错误投影），本仓无既有配置面，可一次到位。

## Decisions

- `internal/config`（viper）：`Keys` 白名单（state/rclone_config/scaena_url/scaena_token_file/remote_url/remote_token_file/local_root）；`Discover` 绝对化 CWD 候选；`Load` 用 `AllKeys` 对白名单严格比对，未知键报 `unknown keys: ...` 退出；`ResolveAll` 每键产出值+来源。
- 优先级：显式旗标（含命令本地旗标，经 `flagNames` 映射参与解析）> `DRIVEBRIDGE_<KEY>` env > 文件 > 内置默认；空显式旗标不遮蔽文件值。
- cobra：root `SilenceUsage/SilenceErrors`，main 集中打印 `DriveBridge failed: <typed code> ...`（doctor not-ready 哨兵静默）；`PersistentPreRunE` 统一解析配置；openService 保持旧路径纪律（mkdir 0700→flock→rclone 路径→bridge.New→日志静默）。
- `space-add` 保留为隐藏 deprecated 别名；新用法 `space add`/`space list`。`config init` 写模板（0700 目录/0600 文件、拒绝覆盖）；`config show` 不含任何 secret（值只可能是路径/URL）。
- `config init` 容忍显式 `--config` 指向不存在文件（那是它的目标路径）；其余命令遇显式缺失路径立即失败（防路径打错静默回退默认）。
- doctor `config_file` 检查永远 info/pass：配置可选；文件含组/其他读权限时附 0600 建议。

## Interfaces

`internal/config/config.go`（Discover/Load/ResolveAll/Template/EnvName）；`cmd/drivebridge/{main,root,commands,doctor}.go`（cobra 树）；旧行为等价面：`capabilities/space-add/mcp/remote/auth/doctor` 旗标与输出合同零变化。

## Verification / Rollback

Go 单测（发现序/严格键/优先级/空旗标透传）+ testscript（init 拒绝覆盖、show 来源、env/flag 压栈、未知键拒绝、配置驱动 space-add 落库、space list）+ check.py（race/vet/CGO-free）+ 真实 stdio 冒烟。回滚：不提供配置文件即旧行为；cobra 旗标名不变，注册命令行无需改。

## 去向

实现随本 change 完成；seamless-auth 实现波的 device flow 挂 `auth` 子命令下。
