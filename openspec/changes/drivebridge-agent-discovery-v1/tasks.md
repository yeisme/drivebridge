## 1. 实施与验证

- [x] 1.1 完成 search/changes 游标/内容寻址/TTL 保留的设计与合同
- [x] 1.2 实现波（批次一：search+changes 游标；批次二：内容寻址秒传+--link-ref+手动 gc+pin）
- [x] 1.3 实现批次一：search（索引+垃圾过滤+stat 纪律）+changes 游标（Event 表单调序号/零游标不回放/cursor_ahead）
- [x] 1.4 实现批次二：内容寻址秒传+--link-ref+手动 gc+pin

验证证据：`temp/integration-test-runs/20260912T190438Z-9f168eb8/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
