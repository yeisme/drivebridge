## Context

`Open` 在启动时强校验 state（密钥缺失而库存在即拒绝启动），但操作者只有在 serve 时才看到失败；部署清单也没有可勾选的单一入口。

## Decisions

- doctor 为只读命令：不 `MkdirAll`、不取 `process.lock`、不调用 `service.Open`（其副作用含 interrupted 改标与 repairParts），不写任何文件。
- 检查项按名输出 `pass/fail/info`：
  - `state_dir`：存在、目录、0700（不创建）。
  - `instance_key` / `instance_id` / `metadata_db`：存在、内容形状（32B key、48 hex 实例 ID）、0600。
  - `state_consistency`：复刻 `Open` 的拒绝规则并给恢复指引——库存在而密钥缺失 → `instance_key_missing_restore_required`（从秘密备份恢复，禁止重新生成掩盖）；库存在而 instance.id 缺失同理；全新空目录 → info（首次 serve 会初始化）。
  - `web_dir`：传入 `--web-dir` 时要求目录内存在 `index.html`。
  - `platform_storage`：只读查询 `connections` 中 `id='platform'`；缺失 → 指向 `platform-connect`；disabled → fail。
  - `oidc_issuer`：传入 `--issuer` 时 GET `<issuer>/.well-known/openid-configuration`（10s 超时），返回 JSON 且 `issuer` 字段与配置一致才 pass；探测失败只失败本项，不中断其余检查；不做签名/账号级验证。
- 数据库读取用 `mode=ro` 打开 `metadata.sqlite`；无库时跳过依赖库的检查项。
- 输出沿用 CLI 输出合同（`--json/--agent/--events/--explain`）；not ready 打印明细后以非 0 退出（哨兵错误）；`--agent` 补标量 `fact.*`（emit 增加 data 支持，platform-connect/backup 行为不变）。

## Interfaces

`cmd/drivebridge-server/doctor.go`：`buildDoctorReport`（纯函数、可单测）+ `runDoctor`。`outputMode.emit` 增加可选 data（nil 时与旧行为逐字节一致）。

## Verification / Rollback

Go 单测（fixture state 三件、恢复导向失败、httptest issuer discovery、web-dir 检查）+ `scripts/check.py` 全绿；证据入 `temp/integration-test-runs/`。回滚：删除 doctor 子命令与文档；emit 的 data 参数为向后兼容扩展。
