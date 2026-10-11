# 文件交接查询与恢复

## 能力归属

本项目入口为 fit，整体创作文件旅程为 split-owner。Eikona 拥有图片输入、审核与选集，本机 DriveBridge 拥有电脑文件读取和主动中转，Server 拥有文件版本、权限与 grant。

## 决策

为 input.bind_reference 增加可选 binding_id、consumer_request_ref 和 RFC3339 request_expires_at，保留旧必填项及返回字段。增加 input.handoff_status/renew/cancel 与 handoff status/renew/cancel CLI，复用既有 service/SDK。统一身份的状态查询属于只读；续期、取消及字节获取保留原准入。

配置存在不表示网络已验证。失败先查原记录，不改身份、目的地或幂等键，不触发生图。只读发现不签发权限。

## 验证

```bash
python3 scripts/check.py -- go test -race ./...
go vet ./...
openspec validate drivebridge-handoff-interaction-v1 --strict --no-interactive
```

聚焦测试覆盖真实 CLI 请求、MCP 分派、原标识保持、终态、权限拒绝与输出脱敏；最终结果和证据写回 tasks.md。

## 兼容和回滚

仅新增可选字段与动作；原数组、URI、命令和权限不变。可停用新增入口而不删除操作记录。没有自动重试器或凭据配置变更。独立 v0.8.1 候选保持冻结。
