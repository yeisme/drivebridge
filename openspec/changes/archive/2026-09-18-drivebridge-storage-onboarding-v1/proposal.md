## Why

根包 `openspec/changes/drivebridge-storage-onboarding-v1`（仓库根）冻结了接入引导合同。本包落地：`storage kinds/probe/add` 命令、capabilities 加法键、三个错误码与 recipe 文档。

## What Changes

- `internal/bridge/storagekinds.go`：kind 目录、`ProbeStorage`（只读列举）、`EnsureAliasRemote`/`EnsureWebDAVRemote`（rclone 配置写入，密码 obscure）。
- `cmd/drivebridge/storage.go`：三个命令；`storage add` 校验→自动 remote→探测→委托 `spaceAddCommand(c,"storage-add")`。
- capabilities JSON：`storage_kinds`、`storage_add`（命令模板）。
- `internal/recovery`：`storage_kind_unknown`、`storage_missing_flag`、`probe_failed`；`docs/error-codes.md` 同步。
- `testdata/storage.txt`：kinds/未知 kind/缺旗标/probe 失败与通过/local kind 端到端。
- `docs/storage-recipes.md`：gdrive、openlist、s3/minio、smb/sftp、local/alias recipe 卡。

不修改 `space add`/`setup` 行为；不动 OneDrive 交接。

## Capabilities

### New Capabilities
- `drivebridge-storage-onboarding`（对齐根包同名能力）。

### Modified Capabilities
- `drivebridge-local-files`：capabilities JSON 加法键（旧客户端忽略）。

## Impact

Owner：`mcp/drivebridge`。验证：`python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、本包 strict 校验。
