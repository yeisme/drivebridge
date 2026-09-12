## 1. 实施与验证

- [x] 1.1 完成 search/changes 游标/内容寻址/TTL 保留的设计与合同
- [ ] 1.2 实现波（批次一：search+changes 游标；批次二：内容寻址秒传+--link-ref+手动 gc+pin）

验证证据：`temp/integration-test-runs/20260912T133908Z-f6babdb4/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
