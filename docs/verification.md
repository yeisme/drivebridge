# 本地验证记录

本轮实现为 pre-1.0 本地候选；没有真实账号操作、付费生成、部署或发布。

- `temp/integration-test-runs/20260911T041805Z-5bc71cf5/`：Go/race、CLI、Graph/OAuth、路径与身份隔离、版本、取消、上传回执丢失恢复、MP4 校验、成品回存。
- `temp/integration-test-runs/20260911T041812Z-4cc6fa40/`：真实 stdio tools/list、能力资源 schema 一致性与默认关闭。
- `temp/integration-test-runs/20260911T041814Z-5d28ffe4/`：go vet。
- `temp/integration-test-runs/20260911T041815Z-6d2259ff/`：CGO_ENABLED=0 构建。

MCP schema 初次比较因 tools/list 的排序不同失败，调整为按工具名称比较完整定义后通过。测试未弱化字段/schema 的比较。失败证据保留。

Scaena 专项验证见对应 owner 文档。真实 Gateway、OneDrive 和火山区域队列有效期不在本地验证结果中。

新增私有 state/socket 目录检查后，旧测试临时目录权限过宽被正确拒绝；修正测试目录权限并新增拒绝公开 state 的断言后，最终测试全部通过。

## activation-readiness（2026-09-11）

- `temp/integration-test-runs/20260911T143824Z-89c33284/`：go test -race 全量（含 doctor 单测与 testscript 两态用例）。
- `temp/integration-test-runs/20260911T143838Z-15594600/`：go vet。
- `temp/integration-test-runs/20260911T143838Z-bb619b66/`：CGO_ENABLED=0 构建。

doctor 预检只读：不创建 state、不取进程锁、`mode=ro` 打开 state.sqlite；HTTP 探测只证明可达，授权一律 not probed。agent 模式补充标量 `fact.*` 输出（排序键，嵌套只在 `--json`）。真实账号冒烟按 [activation-local.md](activation-local.md) 由用户本人执行后另记。
