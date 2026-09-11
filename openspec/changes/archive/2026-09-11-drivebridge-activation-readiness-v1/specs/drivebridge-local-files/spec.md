## ADDED Requirements

### Requirement: Preflight readiness reporting
doctor SHALL 以只读方式报告激活所需预检项，按名区分 pass/fail/info；不得获取写锁、发起真实账号探测，或在输出中包含凭据、临时 URL 或原始第三方错误。

#### Scenario: All prerequisites met
- **WHEN** 本机预检全部通过（私有 state、rclone 远端、空间、可写 local root、ffprobe，及可选的 handoff socket 与 Scaena 端点）
- **THEN** 命令汇总 ready 并以 0 退出

#### Scenario: Missing or broken prerequisite
- **WHEN** 任一预检项缺失或损坏（无空间、local root 不可写、残留 socket、凭据文件权限过宽等）
- **THEN** 按名报告失败项并以非 0 退出，明细不泄露 token、URL 或原始错误正文

#### Scenario: Remote mode preflight
- **WHEN** 以 `--remote-url` 运行 doctor
- **THEN** 校验服务地址形状、token 文件权限与 local root，并探测 `/healthz` 可达性；可达不代表授权有效，授权状态记为 not probed
