## 1. 实施与验证

- [x] 1.1 实现 doctor 只读部署预检（state 三件一致性/恢复指引、web-dir、平台存储、issuer discovery）与 emit data 扩展
- [x] 1.2 编写 docs/deployment.md 生产部署 runbook 并修复 docs/README.md 归档链接
- [x] 1.3 doctor 单测（fixture state、密钥缺失恢复指引、issuer 探测、web-dir）全绿
- [x] 1.4 check.py（race/vet/CGO-free 构建）与 openspec validate --strict 全绿

验证证据：`temp/integration-test-runs/20260911T144509Z-6c7233af/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
