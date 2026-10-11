## Why

错误面是交互体验第一债：HTTP 失败信封只有 `error.code` 无 message（`invalid_request`/`invalid_upload` 零补救线索）；多个服务端失败码（storage_delete_failed、delivery_unavailable、throttled、transfer_failed、export_conflict 等）全部落 400 默认，重试/退避判断被误导；`user_import_failed` 同时掩盖重复 key 与 DB 失败；`storage_read_failed` 在 OneDrive 导出路径被复用于非存储读失败；CLI 失败路径绕过输出信封（--json 出空 stdout）；audit list 人读模式无行输出；platform-connect --verify-isolation 发两个 JSON 对象违反单对象契约。

## What Changes

- HTTP 错误信封增加静态英文 `message`（注册表驱动，不插值原始错误）；状态映射修正：onedrive_authorization_required→401、object_missing→404、user_key_duplicate/export_conflict→409、throttled→429、storage_delete_failed/transfer_failed/delivery_unavailable→503。
- `user_import_failed` 拆分为 `user_key_duplicate`（重复哈希，整批不落库）与 `database_unavailable`；runExport 本地读失败改报 `transfer_failed`。
- SDK 错误类型化：`*sdk.Error{Code,Message}`，`Error()` 仍返回裸码（字节兼容），调用方可读 message。
- MCP 工具 schema 真实化（trash/restore 声明 `ref`、insight 工具逐字段描述、防漂移测试）——与并行 lane 的 `input.bind_reference` 同文件，随该 lane 统一落地。
- CLI：失败经 emitFailure 出真实码信封；audit 人读模式打行；platform-connect 折叠单信封（over_scoped→status=partial，`data.storage_isolation` 保留）；doctor not-ready 以退出码 2 与硬错误区分。
- doctor 新增 login_identities 预检：无可登录 key 且无 OIDC issuer 时 fail 并给出恢复命令。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-service`：Safe user and Agent interfaces（错误信封 message+映射+SDK typed error）、Operator-issued user keys（重复码拆分）、Deployment preflight（可登录身份预检）、Storage isolation verification probe（单信封）、Configured deployment surface（CLI 失败信封/audit 行/退出码）。

## Impact

backend-server/drivebridge；client 仓补新码文案（另一小改）。`error.message` 为增量可选字段（minor）；状态码修正属 bug 修复（原值不在任何已文档语义内）；platform-connect 单信封是对既违契约的合规修复（原 --json 两对象），README 记迁移注记。
