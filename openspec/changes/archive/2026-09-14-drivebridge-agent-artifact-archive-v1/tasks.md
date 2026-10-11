## 1. 实施与验证

- [x] 1.1 收件箱合同：HTTP/MCP 近 30 天操作查询、status/caller/space 过滤、读权限后置过滤、database_unavailable 故障策略
- [x] 1.2 受管归档：固定版本服务端副本、冲突安全改名、幂等键、resume 按记录源版本重传、SDK Inbox/Archive 方法
- [x] 1.3 验收：单测（固定版本钉住与冲突改名）与全量包测试经 evidence runner 通过

验证证据：`temp/integration-test-runs/20260914T152633Z-cf7f0f23/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
