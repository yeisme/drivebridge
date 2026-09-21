# 任务

- [x] 1.1 Overlay 规划器：控制面排除、空内容根创建 `notes`、Capsa 检测、冲突。验证：`go test ./internal/bridge -run Overlay -count=1` Evidence: PASS
- [x] 1.2 `mount vault --preset pinax` 与 `preset pinax-vault` CLI、status 加法字段、recovery 码与 error-codes 表。验证：`go test ./cmd/drivebridge -count=1` 含 testdata Evidence: TestCLI PASS including testdata/pinax-vault.txt
- [x] 1.3 capabilities `mount` 键区分路径预设；Agent 无 start。验证：testdata `cli_only` Evidence: mount.txt and pinax-vault.txt
- [x] 1.4 `CGO_ENABLED=0 go build ./cmd/drivebridge`；`openspec validate drivebridge-pinax-vault-mount-v1 --strict --no-interactive` Evidence: both PASS
- [x] 1.5 S3 rclone 路径适配：`type=s3` → `rclone mount` 内容根 + stop 时 umount。验证：`go test ./internal/bridge -run 'RcloneMount|PrepareSpace' -count=1`；`go test ./cmd/drivebridge -run TestCLI -count=1` 含 testdata/pinax-vault-s3.txt Evidence: unit + TestCLI/pinax-vault-s3 PASS；CGO=0 build PASS
