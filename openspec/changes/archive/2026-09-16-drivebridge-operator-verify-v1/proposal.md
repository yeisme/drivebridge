## Why

签发出去的 key 无法确认：操作员拿到一张 dist.csv 后，没有一条命令回答「这把 key 到底 live/revoked/expired」——泄露处置与轮换前的核对全靠盲信。serve 的就绪信号只有一行 stderr 人读文本，监督进程（init/supervisor/agent）无法机读「何时可接受流量」。

## What Changes

- `users verify --key <plaintext>`：本地哈希查（无网络、无认证副作用），报告 id/name/state(live|revoked|expired)/admin/expires。
- `serve --events`：唯一放行的机器模式（--json/--agent/--explain 仍拒）；NDJSON start→progress(ready，在 listener 真实 bind 之后)→end/error，日志仍走 stderr。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-service`：ADDED 需求 Operator key verification；Configured deployment surface 增加 serve 监督流场景。

## Impact

backend-server/drivebridge；verify 属 operator 面（存在性披露按 operator 权限）；serve 行为对既有调用不变（默认模式输出与 stderr ready 行保留）。
