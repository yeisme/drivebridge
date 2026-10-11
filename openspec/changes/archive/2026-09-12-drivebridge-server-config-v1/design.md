## Context

与 mcp 仓 drivebridge-cli-config-v1 同一套模式（aigora viper+scaena cobra 先例），键集换成部署面：state/listen/origin/issuer/client_id/client_secret_file/onedrive_client_id/onedrive_secret_file/web_dir/max_bytes/tenant_name/tenant_origin/tenant_contact。

## Decisions

- `internal/config`（server 变体）：发现序末尾加 `/etc/drivebridge-server/config.*`（自托管服务器惯例）；Load 除白名单外校验 origin/issuer 必须为无路径 origin、max_bytes 正整数；`Effective.Int` 解析整型键带回退。
- cobra：main 薄壳 + `cli.resolve`（PersistentPreRunE 统一解析；`config init` 容忍显式目标缺失）；**AddCommand 必须传指针**——按值复制 cobra.Command 会断子命令 parent 链导致继承持久旗标解析失败（本轮实踩坑）；serve 命令旗标仅作解析目标，值全部经 `c.effective` 取。
- serviceConfig：Enabled/Development 只来自命令行布尔旗标；client secret 仍经 readSecret 读 0600 文件（配置只存路径）。
- doctor 走只读分支（不 Open）；`config_file` 检查 info/pass 永不阻塞。
- 输出合同：emit/emitData 不变；`--agent` 标量 fact.*（上一波已加）；serve 拒绝输出模式（原语义）。

## Interfaces

`internal/config/config.go`（Keys 白名单/Discover/Load/ResolveAll/Int/Template）；`cmd/drivebridge-server/{main,root,commands,doctor}.go`（cobra 树，六命令+config+completion）。

## Verification / Rollback

Go 单测（键白名单/形状校验/优先级/Int/模板）+ CLI 级测试（init 0600 与拒覆盖/show 来源/未知键全命令拒绝/doctor 报 config_file）+ 既有 doctor/backup/加密套件零改动 + fixture 冒烟（config 驱动 serve → healthz 200）+ check.py（race/vet/CGO-free）。回滚：无配置文件=旧行为；旗标名不变。

## 去向

实现随本 change 完成；seamless 登录/bisync 仍归 mcp 仓后续波。
