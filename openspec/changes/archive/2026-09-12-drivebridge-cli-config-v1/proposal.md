## Why

CLI 是平铺 flag.FlagSet：20+ 旗标每次命令重复传（--state/--rclone-config/--scaena-*），MCP 客户端注册是长命令行；无配置文件、无子命令帮助、无 completion。个人日常使用的主摩擦面在命令行形态本身。

## What Changes

- 新增可选配置文件（viper）：发现序 `--config` > `DRIVEBRIDGE_CONFIG` > `./drivebridge.{yaml,yml,json,toml}` > 用户配置目录；优先级 显式旗标 > `DRIVEBRIDGE_*` env > 文件 > 默认；**未知键拒绝**（打错键名立即失败）。
- cobra 命令树：`space add/list`（旧 `space-add` 保留为隐藏别名）、`auth`、`doctor`、`capabilities`、`mcp`、`remote`、`config show/init`、`completion`；持久旗标 `--config/--state/--rclone-config` 与四个输出旗标。
- 安全边界固化：`--enabled/--handoff` 等写入口开关**不入配置文件**；secret 永不入文件。
- doctor 新增 `config_file` 检查（info 级）；`config show` 报告每键来源（flag/env/file/default）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-local-files`：新增 Configured CLI surface 需求。

## Impact

旗标名全部不变；无配置文件时行为与旧 CLI 等价（纯默认）。testscript usage 断言随 cobra 帮助格式更新；根 conformance needle 不受影响（`"doctor"`/`errDoctorNotReady` 仍在 main.go）。
