## 1. 实施与验证

- [x] 1.1 `platform_unix.go` / `platform_windows.go`：state 锁（flock ↔ LockFileEx）与可写探测（Access ↔ 探针文件）平台抽象
- [x] 1.2 view 浏览器启动三平台分支（xdg-open / open / cmd start）
- [x] 1.3 `build.py` windows/amd64+arm64 目标、zip 打包、manifest 状态 `local-candidate-windows-unverified-on-machine`
- [x] 1.4 验证：`GOOS=windows`（amd64+arm64）编译、`go vet`、`go test ./cmd/drivebridge`、真实 `build.py --target windows/amd64` 打包内容校验（exe + web 资产）
- [x] 1.5 README / personal-agent-loop 平台措辞更新（Windows=候选，未实机验收）
- [x] 1.6 `openspec validate drivebridge-cross-platform-distribution-v1 --strict --no-interactive`

## 2. 实施与验证

- [ ] 2.1 Windows 实机验收（state 权限语义/AF_UNIX socket 隐私/浏览器启动/rclone+ffprobe）——需要真实 Windows 机器；2026-09-16 决策：延期至出现真实 Windows 用户，分发保持 experimental 候选，届时重开 change 验收
- [x] 2.2 跨平台设计文档（归 client owner change 撰写，本任务仅登记交接）

验证证据：`temp/integration-test-runs/20260914T153248Z-957211c5/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
