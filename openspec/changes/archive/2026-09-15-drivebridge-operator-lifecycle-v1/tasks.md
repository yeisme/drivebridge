## 3. 实施与验证

- [x] 3.1 status 命令：远程（healthz/me/spaces 单投影、0600 token 文件、逐跳专属失败码）+ 本地（ro 快照聚合、恰好一条下一步建议）；三测试
- [x] 3.2 agents create/list：owner 从空间行推导、createAgent 原样复用、一次性令牌 stdout/--out 二选一、list 无机密；服务级+CLI 级测试
- [x] 3.3 运营文档：四步 runbook 推荐 --out+新增 ⑤ status 核对、agents 直发说明、配额口径（max-used-reserved）入 deployment/contract-map

验证证据：`temp/integration-test-runs/20260915T153002Z-c0c72b67/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
