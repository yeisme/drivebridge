# 本机中转目标发现

## 能力归属

本项目入口为 fit，整体创作文件旅程为 split-owner。Eikona 拥有图片输入、审核与选集，本机 DriveBridge 拥有电脑文件读取和主动中转，Server 拥有文件版本、权限与 grant。

## 决策

新增只读 relay_targets，使用现有 RelayTarget 安全投影。返回 ID、源空间、项目、disabled 和 not_probed；隐藏端点和凭据文件路径。CLI 明确必要配置选项，原 targets 数组输出不变。

配置存在不表示网络已验证。失败先查原记录，不改身份、目的地或幂等键，不触发生图。只读发现不签发权限。

## 验证

```bash
python3 scripts/check.py -- go test -race ./...
go vet ./...
openspec validate drivebridge-relay-discovery-v1 --strict --no-interactive
```

聚焦测试覆盖真实 CLI 请求、MCP 分派、原标识保持、终态、权限拒绝与输出脱敏；最终结果和证据写回 tasks.md。

## 兼容和回滚

仅新增可选字段与动作；原数组、URI、命令和权限不变。可停用新增入口而不删除操作记录。没有自动重试器或凭据配置变更。独立 v0.8.1 候选保持冻结。
