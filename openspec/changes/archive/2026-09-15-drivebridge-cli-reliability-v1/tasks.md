## 1. 实施与验证

- [x] 1.1 失败契约：fail() 五模式渲染（真实 error.code、events start 带 spec_version、agent error.code）、main 分流（errQuiet/codify 兜底）、insights/sync 双打消除、传输终态归一；output_test 表驱动
- [x] 1.2 锁共存：state_locked 类型化+250ms 重试、readonly.go 快照回退 recent/operation（read_only 标注）；recent_ro_test 三用例
- [x] 1.3 错误码注册表：codes.go 单源（hintEN/docZH）+ docs/error-codes.md + golden 同步；cmd 层裸英文错误全部类型化（含 install/config/sync 拒绝详情入 report.refusal_reason）
- [x] 1.4 首跑平滑：config set 默认 config.yaml、handoff socket dial-probe 自愈、裸 mcp 无 --enabled 启动警告、doctor state_dir_missing 确切命令提示
- [x] 1.5 传输进度与扁平化：eventStream（start→progress 节流→end/error，seq 单调）、人读 stderr 单行进度、--agent 嵌套一层扁平+_count；两用例

验证证据：`temp/integration-test-runs/20260915T145214Z-c46ff46c/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
