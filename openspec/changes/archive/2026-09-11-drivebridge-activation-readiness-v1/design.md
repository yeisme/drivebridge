## Context

v1 归档后全部验证停留在 fixture 层。README 描述了真实激活所需步骤（auth、space-add、MCP 注册、Scaena 环境变量），但没有可执行清单，也没有在激活前/故障时一条命令看清缺什么。

## Decisions

- doctor 为只读命令：不 MkdirAll、不取 `process.lock`、不调用 `bridge.New`（避免把进行中操作改标 interrupted），以 `mode=ro` 打开 `state.sqlite` 枚举空间。
- 检查项按名输出 `pass|fail|info`：
  - 本机模式：state 目录 0700、rclone 配置存在且组/其他不可读、空间非空且 `space.remote` 出现在 rclone 配置节、`space.local_root` 存在且可写、ffprobe 可用、handoff socket 存活（未启动为 info；`--handoff` 声明意图时缺失为 fail）、可选 Scaena 端点形状/token 文件权限/可达性；OneDrive 云区域固定 global 仅作 info。
  - 远程模式：remote-url 形状（https 或回环 http）、token 文件 0600、local-root 绝对且可写、`/healthz` 可达。
- 诚实就绪：HTTP 可达与配置存在不代表授权有效，授权一律 `not probed`；不发起任何 Graph/真实账号请求，不打印 token、临时 URL 或原始第三方错误。
- 输出沿用 CLI 输出合同：默认人类可读逐项明细 + `--json/--agent/--events/--explain`；not ready 打印明细后以非 0 退出（哨兵错误，跳过通用失败横幅）。`--agent` 模式为标量 `fact.*` 键补充排序输出，嵌套结构只进 `--json`。

## Interfaces

`cmd/drivebridge/doctor.go`：`buildDoctorReport`（纯函数、可单测）+ `runDoctor`（打印与退出哨兵 `errDoctorNotReady`）。远程/本机 URL 形状校验与 `sdk.New`、`NewScaenaOutput` 同规则（内联小函数，不引依赖）。

## Verification / Rollback

Go 单测（远程模式 httptest、本机报告构建）+ testscript CLI 两态用例（fresh not-ready / 完整 ready / 残留 socket / state 权限过宽）+ `python3 scripts/check.py` 全绿；证据入 `temp/integration-test-runs/`。回滚：删除 doctor 子命令与文档即可，无状态迁移、无默认行为变化。
