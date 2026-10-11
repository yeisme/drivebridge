## 1. 实施与验证

- [x] 1.1 `users import --csv`（表头/无表头/单列名单解析 + BOM/空行容忍）与 `--out` 分发 CSV（0600）；JSON 路径回归不变
- [x] 1.2 `scripts/issue_users.py`：本地 CLI 发 key + 可选 LiteLLM 列（env 门控），合并输出 CSV；含模板 `scripts/users-template.csv`
- [x] 1.3 单测（CSV 解析矩阵/坏行整批拒绝）+ check.py 包裹真机跑（含 --out 权限）；`go vet`、`CGO_ENABLED=0 go build` 全绿
- [x] 1.4 部署 runbook 填表配方

验证证据：`temp/integration-test-runs/20260915T134708Z-3f21366c/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
