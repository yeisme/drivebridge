## Why

CLI 已有四输出模式旗标，但失败路径完全绕过输出机制：`--json` 承诺信封却在出错时 stdout 全空、stderr 散文；信封错误码恒为占位 `command_failed`。排他进程锁让注册了 MCP 的用户一碰 CLI 传输命令就吃到裸 OS 错误。约一百个错误码没有注册表与翻译（仅 recent/resume/cancel 三命令有恢复提示）。首跑还有一圈易绊点（配置文件名分裂、陈旧 socket、裸 mcp 假活、doctor 处女机提示含糊）。

## What Changes

- **失败契约**：全部命令错误经输出机制渲染——机器模式出合法失败信封（真实 `error.code`+message、--events 为 start+error 且 start 带 spec_version），人读模式 stderr 一行 + 注册表恢复提示；第三方原始文本绝不越过边界（codify 兜底 command_failed）。传输终态归一（interrupted/cancelled 不再泄漏进信封 status）。
- **锁共存**：flock 冲突类型化为 `state_locked`（含 250ms 有界重试）；`recent`/`operation` 在持锁时回退只读快照（`data.read_only=true`），其余命令给出可行动提示。
- **错误码注册表**：`codes.go` 单源（hintEN/docZH）驱动人读提示与 `docs/error-codes.md`（golden 同步测试）；cmd 层裸英文错误全部类型化。
- **首跑平滑**：config set 无文件时默认写 config.yaml（与 init/发现序一致）；handoff socket 绑定失败时 dial-probe，确证死亡才回收重试一次；裸 `mcp` 无 --enabled 启动即 stderr 警告点名被门禁工具；doctor state_dir_missing 提示给出确切命令。
- **传输体验**：--events 流式 start→progress（≥1s/≥5% 节流、seq 单调）→end/error；人读模式 stderr 单行刷新进度；--agent 扁平化嵌套 map 一层（fact.operation.status）与 slice 计数。

## Capabilities

### Modified Capabilities

- `drivebridge-local-files`：新增失败契约与错误码注册表需求；收件箱、激活诚实性、doctor 预检、CLI 配置面、工作区传输 CLI 五需求增场景。

## Impact

mcp/drivebridge 全量实现；同步拒绝详情从错误串移入 `SyncReport.refusal_reason`（增量字段）；`--events` start 事件补 spec_version；sync refused→failed、传输终态→success/failed 归一属既有契约的合规修复（原值不在枚举内）。
