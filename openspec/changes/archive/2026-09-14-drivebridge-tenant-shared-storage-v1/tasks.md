## 1. 实施与验证

- [x] 1.1 真实 COS 探测证据归档：SigV4 + 虚拟主机寻址全 CRUD 绿、path-style 拒绝码、region 拼法结论（temp/integration-test-runs）
- [x] 1.2 change 严格校验通过（openspec validate --strict）

## 2. 实施与验证

- [x] 2.1 `S3Config.VirtualHosted` 寻址落地：客户端构造、JSON 键、旧封存零迁移单测
- [x] 2.2 隔离探针可选接口 + `platform-connect --verify-isolation`：保留命名空间两落点、超域清理、结果报告；单测覆盖三种结局
- [x] 2.3 真实 COS 端到端：platform-connect（virtual_hosted）+ 隔离探针超域路径 + 经服务写入/回读/删除共享桶对象；凭据仅经环境变量，证据入 temp/integration-test-runs
- [x] 2.4 部署文档共享桶多部署配方（CAM 前缀作用域子凭据样例、platform-connect 输入样例、验证流程、加密兜底）；`python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server` 全绿

验证证据：`temp/integration-test-runs/20260914T074105Z-f79d602b/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
