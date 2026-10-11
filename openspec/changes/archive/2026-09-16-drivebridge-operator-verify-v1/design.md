# Design: drivebridge-operator-verify-v1

## 决策

1. **verify 走哈希直查而非真认证**：不构造 HTTP 请求、不写审计登录事件——它是对已分发材料的核对，不是一次登录；state 报告叠加 revoked/expires 判定（Go 侧，宽容格式）。
2. **serve ready 事件在真实 bind 之后**：改为 net.Listener 先 bind 再 Serve——"ready" 现在意味着端口真的在听，而不是即将听；启动失败（bind 失败/serve 中断）分别发 error 事件，监督进程可据事件驱动重启。
3. **只放行 --events**：stdout 的其余时间归 HTTP/stderr 日志；end 事件在优雅关停完成后发出。
