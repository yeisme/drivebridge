# Tasks

## 1. 离线彩排能力

- [x] 1.1 `cmd/drivebridge-fixture --s3-listen <addr>`：S3 fixture 独立回环监听，endpoint 打 stderr；不加旗标行为不变
- [x] 1.2 `platform-connect --development`：显式 CLI 旗标（不进配置文件/env），允许回环 HTTP fixture 端点；默认拒绝行为与错误码不变；补 CLI 级测试（回环端点带/不带旗标的成败对照）
- [x] 1.3 `scripts/selfhosted_rehearsal.py`：真实二进制 + 真实 client dist 四步 key-only 彩排 + 浏览器等价 API 冒烟 + 备份只读校验；证据落 `temp/integration-test-runs/<run-id>/`（summary/command/stdout/stderr/env/artifacts），明文 key 只在 0600 artifacts 分发文件
- [x] 1.4 `docs/deployment.md` 增补「自托管激活（Tailscale + key-only + S3-only）」章节：形态决策、首验门、回滚；真实部署步骤单列并标注需操作者输入

## 2. 验证

- [x] 2.1 `python3 scripts/check.py` 全绿（含新增彩排脚本跑通一次）

验证证据：`temp/integration-test-runs/20260916T072227Z-be7812ad/`（go test -race 全量）、`temp/integration-test-runs/20260916T072125Z-79c833f7/`（彩排脚本首跑，artifacts 内含 0600 dist.csv，阅后可清理）。
- [x] 2.2 `python3 scripts/check.py -- go vet ./...` 与 `CGO_ENABLED=0 go build ./cmd/drivebridge-server ./cmd/drivebridge-fixture`
- [x] 2.3 `openspec validate drivebridge-selfhosted-activation-v1 --strict --no-interactive`

## 3. 真实部署（需操作者输入，保持未勾）

- [x] 3.1 操作者提供：服务器访问方式、Tailscale 状态、S3 凭据渠道、目标 OS/arch——实际输入：内网长期服务器（替代 Tailscale 方案）、rclone 既有 COS 配置、linux/amd64
- [x] 3.2 真实服务器四步激活 + 本人设备浏览器首验（按 deployment.md 自托管章节）——内网 TLS（Caddy internal）+ key-only + 真 COS 激活，首验门全过（详见 docs/verification.md 2026-09-16 自托管激活真机验收）；操作者本人设备的浏览器实测与 CA 信任为使用侧动作
