## 1. 实施与验证

- [x] 1.1 generic 桥 inbox 工具：近 30 天完成/失败操作倒序列表（上限 200）、只读不触发传输
- [x] 1.2 state 不可用如实返回 state_unavailable；收件箱顺序与诚实失败两测试

验证证据：`temp/integration-test-runs/20260914T153248Z-957211c5/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
