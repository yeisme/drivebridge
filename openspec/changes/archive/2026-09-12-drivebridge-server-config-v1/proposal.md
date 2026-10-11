## Why

生产部署是 10+ 旗标长命令行（serve --state --origin --issuer --client-id --client-secret-file --onedrive-* --tenant-* --web-dir --listen --max-bytes）；无配置文件、无子命令帮助、无 completion，部署面主摩擦在命令行形态。

## What Changes

- 新增可选部署配置文件（viper）：发现序 `--config` > `DRIVEBRIDGE_SERVER_CONFIG` > `./drivebridge-server.{yaml,yml,json,toml}` > 用户配置目录 > `/etc/drivebridge-server/config.*`；优先级 显式旗标 > `DRIVEBRIDGE_SERVER_*` env > 文件 > 默认；**未知键拒绝** + origin/issuer 形状与 max_bytes 正整数校验。
- cobra 命令树：serve/platform-connect/backup/doctor/key-rotate/space-export 旗标名零变化 + `config show/init` + completion；持久旗标 `--config/--state` 与四输出旗标。
- 安全边界固化：`--enabled/--development` 与 secret 值**不入配置文件**（文件只承载路径与限额）；doctor 新增 `config_file` 检查（info 级）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增 Configured deployment surface 需求。

## Impact

无配置文件时行为与旧 CLI 等价；`doctor` 仍走只读分支（不取实例锁）。旧 usage 行由 cobra help 取代；既有 Go 测试（doctor/backup/加密）零改动通过。
