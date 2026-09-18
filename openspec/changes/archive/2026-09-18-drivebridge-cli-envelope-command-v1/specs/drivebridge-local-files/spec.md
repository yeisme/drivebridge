## MODIFIED Requirements

### Requirement: Machine-mode failure contract

CLI SHALL 在每种输出模式下为命令失败渲染完整投影：`--json` 输出携带真实 `error.code` 与 `message` 的单一失败信封，`--events` 输出 start（含 spec_version）+error 事件（seq 单调），`--agent` 输出必行加 `error.code`，人读模式在 stderr 给出一行失败说明与恰好一条恢复建议；第三方原始错误文本 SHALL 被折叠为稳定码，绝不进入 stdout/stderr。失败信封的 `command` SHALL 与同一命令成功信封的 `command` 一致（按 `CommandPath` 派生；既有连字符 action `space-add`/`storage-add` 保持字面量）。

#### Scenario: Machine mode failure is a valid envelope
- **WHEN** 任一命令在 --json/--agent/--events 下失败
- **THEN** stdout 收到一个合法失败投影，error.code 为真实类型码而非占位符，诊断文本只在 stderr

#### Scenario: Failure command identity matches success
- **WHEN** `space add`、`storage add` 或 `mount status` 失败
- **THEN** 信封 command 分别为 `drivebridge.space-add`、`drivebridge.storage-add`、`drivebridge.mount.status`，与对应成功信封一致

#### Scenario: Third-party text never crosses the boundary
- **WHEN** 底层库返回非类型码错误
- **THEN** 输出使用兜底码 command_failed，原始文本不出现在任何流

#### Scenario: Transfer terminal states are normalized
- **WHEN** 传输以 interrupted/cancelled/failed 结束
- **THEN** 信封 status 归一为 failed 并携带状态派生码（resume_required/operation_cancelled 等），操作明细保留在 data
