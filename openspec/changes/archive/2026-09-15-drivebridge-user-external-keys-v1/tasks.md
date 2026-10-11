## 1. 实施与验证

- [x] 1.1 导入格式放宽（32–128 可打印 ASCII）+ agents 表跨表哈希守卫；MCP 工具描述/错误文案同步
- [x] 1.2 authenticate() bearer 哈希优先路由（UserKey→Agent），删除前缀分支
- [x] 1.3 `users rotate --id [--days]` 原子换发（同 owner 铸新+吊销旧，明文一次）
- [x] 1.4 去 LiteLLM（脚本+runbook）；单测矩阵 + 真机 CLI 证据；check.py/vet/build 全绿
- [x] 1.5 runbook 外部 key 导入配方 + 轮换命令

验证证据：`temp/integration-test-runs/20260915T152856Z-91fcf060/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
