# Design: drivebridge-cli-reliability-v1

## 决策

1. **失败只走一条路**：`write()` 不再接受 failed 状态（删除占位 error 块）；`fail()` 是唯一失败出口，五模式各自渲染。main 在 ExecuteContext 错误分流：`errQuiet`/`errDoctorNotReady` 静默（投影已完整）、cliFailure/类型码走 fail()、第三方原始错误 codify 兜底 `command_failed` 且 stderr 只给通用句。
2. **边界安全复用 insightError 判据**：safeCode（小写/下划线/≤80）与后端同构；详情文本（如 sync 拒绝比例）放 `SyncReport.refusal_reason` 增量字段而非错误串——rclone 引擎文本可能带 URL，永不进错误串。
3. **锁共存按可达性分层**：纯 DB 读（recent/operation）用 doctor 同款 `mode=ro&_pragma=busy_timeout(5000)` 快照（readonly.go）；需要 provider 的命令（ls/stat/find/changes）保持排他 + typed 码——完整 OpenReadonly 构造器留后续。250ms 单次重试吸收 CLI 间接力。
4. **注册表单源**：codes.go 的 map 同时供 hintEN（CLI 人读）与 docs/error-codes.md 表（codes_test golden）；recoveryHint 保留函数名改为查表。
5. **进度流**：eventStream 只在 --events 模式真发射（start 由 stream 构造器发，end/error 收尾，seq 计数器保证单调）；人读模式 stderr `\r` 刷新；其余模式静默。awaitTerminal 保留为无回调包装，recovery.go 行为不变。
6. **socket 自愈的保守判据**：仅 dial-probe 失败（确证无消费者）才 unlink 重试一次；活/未知 socket 维持显式清理规则——原注释的安全意图保留，只消除"死 socket 永久挡路"。

## 备选与弃用

- doctor 自动建 state 目录：spec 禁止 doctor 写状态——弃用，改为提示确切 mkdir 命令。
- `--max-bytes 0` 前置拒绝、`auth --auto` 提示：复核发现已存在——不做。
- 传输并发写共享状态（完整锁改造）：架构级，本波只做类型化+快照读。
