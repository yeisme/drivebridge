## 1. 记录、发现与围栏

- [x] 1.1 依赖无；串行 A。GORM `BindRecord` + `AddBind` 幂等/`bind_mismatch` + purpose 白名单 + 逃生舱拒绝；AutoMigrate。验收：bridge 单测。 | evidence: `go test ./internal/bridge -count=1 -run 'Bind|Discover|Prefix|Adopt'` exit 0
- [x] 1.2 依赖 1.1；串行 A。`DiscoverBindSpaces` 默认只含 bind_default kinds 与已 adopt 空间；`--allow-generic` 才含逃生舱。验收：bridge 单测。 | evidence: TestBindDiscoverDefaultKindsAndAdopted
- [x] 1.3 依赖 1.1；串行 A。BindScope 前缀围栏：scoped list/stat/upload/mkdir/trash 越界 `bind_prefix_denied`；撤销后 `bind_revoked`；未绑定范围不围栏。验收：bridge 单测。 | evidence: TestBindPrefixDeniedAndRevokeStopsReads

## 2. CLI、MCP 与文档

- [x] 2.1 依赖 1.1；串行 A。`bind discover|add|list|show|revoke`；信封 `drivebridge.bind-add` 等；ls/stat/upload/mkdir/trash 加法 `--consumer/--project/--bind`。不删除 storage/connection/relay。验收：CLI bind 测试。 | evidence: `go test ./cmd/drivebridge -count=1 -run 'Bind|Discover|Prefix|Adopt'`；TestBindCLI/bind
- [x] 2.2 依赖 1.2、2.1；串行 A。MCP 加法 `bind_discover`/`bind_list` 与 capabilities；注册错误码并同步 `docs/error-codes.md`；recipe 增加 bind 段。验收：MCP/错误码测试。 | evidence: TestBindMCPDiscoverList；TestErrorCodesDocInSync；TestCLI/storage

## 3. 校验

- [x] 3.1 依赖 1.3、2.2；串行 V。聚焦测试与 `openspec validate drivebridge-consumer-bind-v1 --strict --no-interactive`；`CGO_ENABLED=0 go build`。 | evidence: strict valid；`CGO_ENABLED=0 go build -o /tmp/drivebridge-bind-build ./cmd/drivebridge` exit 0
