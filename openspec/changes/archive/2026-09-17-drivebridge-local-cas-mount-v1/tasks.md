# 实施与验收任务

状态由 scripts/openspec-tasks 维护。全部任务已完成；原始验收证据见各任务 evidence 与文末 run 记录。

Lane 说明：
- lane A（实现）：internal/bridge 的 CAS 与上传去重，1.1→1.2 串行。
- lane B（挂载面）：cmd/drivebridge 的 WebDAV/FUSE CLI 与 MCP 只读状态，依赖 1.1 的 VFS 数据源，与 1.2 可并行。
- lane V（验证）：全量回归与证据，依赖 lane A+B。
- lane D（文档/收尾）：docs 与 strict validation，依赖 lane V。

## 1. 本机 CAS 缓存与下载去重（lane A）

- [x] 1.1 本机 CAS 缓存与 download 去重，version 变化拒绝旧缓存；owner: mcp/drivebridge；范围: `internal/bridge/local_files.go`（download 路径）、新增 `internal/bridge/cas.go`（`<local_root>/.drivebridge-cas/sha256/<aa>/<bb>/<sha256>`，文件 0600、目录 0700、临时文件+rename 发布）；依赖: 无；对应 requirement「本机下载 CAS」及「Agent discovery surface」的 Content-addressed download 场景。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./cmd/drivebridge -count=1`
  - 预期：二次下载零字节且摘要一致；缓存命中仍 stat 远端 version，version 变化不发布旧字节；`.drivebridge-cas` 不出现在 list/search 用户树。
  - 失败复查：先确认失败是摘要计算、发布原子性还是 version 校验；不得为通过测试跳过远端 stat。
  - evidence: 20260917T150326Z-8692b3f8 全量通过；TestDownloadCASDedupeSecondCopy 断言二次下载零字节、命中不拉远端、cache_hit 回执；TestDownloadCASRejectsStaleVersion 断言 version 变化在入队前拒绝且不发布旧字节；TestCASPathHiddenFromUserTree 覆盖 list 与挂载树。

## 2. 远程秒传消费（lane A）

- [x] 1.2 远程 upload 消费 Server `blob_reused`，本机 `ReuseUpload` 行为不变；owner: mcp/drivebridge；范围: `internal/bridge/transfer.go`（upload 声明 sha256）、`internal/remote/remote.go`（Server 命中响应处理）；依赖: 1.1；对应 requirement「Agent discovery surface」的 Content-addressed upload 场景。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./cmd/drivebridge -count=1`
  - 预期：覆盖命中（本机不发分片）/未命中（正常分片）/`idempotency_conflict`；不把他人摘要当作授权；本机 OneDrive 同空间 copy 优化回归不变。
  - 失败复查：用 httptest 记录分片请求计数；`idempotency_conflict` 失败先核对幂等键构造。
  - evidence: 20260917T150421Z-579b2b65 通过；TestRemoteBlobReuseSkipsParts 断言命中零分片，TestRemoteBlobMissKeepsPartsAndConflictPassesThrough 断言未命中走原分片且 idempotency_conflict 原码透传；ReuseUpload 同空间 copy 回归包含在全量中。

## 3. WebDAV 挂载与 MCP 只读状态（lane B，与分组 2 可并行）

- [x] 1.3 WebDAV 只读挂载 CLI + `mount_status`；owner: mcp/drivebridge；范围: 新增 `cmd/drivebridge/mount.go`、`internal/bridge` 新增 WebDAV 适配器（PROPFIND→Readdir、GET/HEAD→Open 含 Range、MKCOL→Mkdir、PUT→Create writable retain-next、MOVE→Rename、DELETE→trash、LOCK→`unsupported`）、`internal/bridge/mcp.go`（`mount_status`）；依赖: 1.1；对应 requirement「WebDAV 挂载默认只读回环」「VFS 操作映射」「MCP 只读挂载状态」。
  - 验证：`python3 scripts/check.py -- go test ./internal/bridge ./cmd/drivebridge -count=1`
  - 预期：默认回环只读且需显式空间列表；非回环无 `--lan` → `listen_not_allowed`；同 state 第二挂载 → `mount_already_running`；未授权空间不可见且不泄露文件名；DELETE 进 trash；MCP tools/list 无 `mount_start`/`mount_stop`，Agent token 不能认证 WebDAV；`.drivebridge-cas`/trash/output 隐藏。
  - 失败复查：权限失败先核对投影的 space_ids；Range 失败核对 Open 的 version 冻结。
  - evidence: 20260917T150406Z-e92e0ae0 通过；TestWebDAVRootListsOnlyAuthorizedSpaces、TestWebDAVAuthAndAgentTokenRejected、TestWebDAVRangeGetFreezesVersion、TestWebDAVReadOnlyAndLockUnsupported、TestWebDAVWritableDeleteMapsToTrash、TestMountStatusReadOnlyProjection 覆盖未授权空间不可见、Agent token 拒绝、Range、只读默认、DELETE→trash、MCP 无 mount_start/stop 且投影不含 token。
- [x] 1.4 默认二进制 `mount fuse` 返回 `unsupported`；owner: mcp/drivebridge；范围: `cmd/drivebridge/mount.go`（fuse 子命令）、可选 `//go:build fuse` 文件；依赖: 1.3；对应 requirement「WebDAV 挂载默认只读回环」的默认 FUSE 场景。
  - 验证：`CGO_ENABLED=0 go build ./cmd/drivebridge` 后运行 `drivebridge mount fuse`（或在 `go test ./cmd/drivebridge -run Fuse` 中断言）
  - 预期：默认构建以 `unsupported` 失败并提示 WebDAV；fuse build tag 不进入默认构建依赖；能力资源报告 `mount.start=cli_only`，新增 `cas_reuse` 与 `mount` 能力项。
  - 失败复查：确认失败不是 libfuse 被意外链入默认构建；检查 `go list -deps` 无 cgo 包。
  - evidence: testscript TestCLI/mount（20260917T150406Z-e92e0ae0）断言默认二进制 mount fuse 以 unsupported 失败并提示 WebDAV；CGO_ENABLED=0 构建通过（20260917T150436Z-8b1c4857）；go list -deps 无 libfuse；能力资源报告 mount.start=cli_only 并含 cas_reuse 与 mount 项。

## 4. 全量回归与证据（lane V）

- [x] 1.5 全量回归；owner: mcp/drivebridge；范围: 全仓；依赖: 1.1–1.4。
  - 验证：`python3 scripts/check.py -- go test ./... -count=1`
  - 预期：全部通过；证据六件套写入 `temp/integration-test-runs/<run-id>/`，失败脱敏并保持退出码；重点覆盖缓存命中、version 变化拒绝、WebDAV 未授权空间不可见、默认 fuse `unsupported`、MCP 无挂载写工具。
  - 失败复查：失败先归因本次引入/既有/环境问题，只修本 change 引入项。
  - evidence: 20260917T150326Z-8692b3f8（全量）与 20260917T150525Z-4de29166（race）均六件套齐、退出码 0；缓存命中、version 变化拒绝、未授权空间不可见、默认 fuse unsupported、MCP 无挂载写工具全部有对应断言。

## 5. 文档与收尾质量门（lane D）

- [x] 1.6 同步文档与最终校验，openspec archive 前完整验证；owner: mcp/drivebridge；范围: `docs/local-cas-mount.md`（CAS 布局、WebDAV 生命周期、rclone mount 指向 WebDAV 的回退说明、回滚）、能力资源说明；依赖: 1.5。
  - 验证：`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`、`openspec validate drivebridge-local-cas-mount-v1 --strict --no-interactive`
  - 预期：三项全过；文档明确回滚路径（停 WebDAV 进程即卸载、CAS 目录可留、关闭远程秒传后 upload 走分片）；不宣称 Server blob 表归属或公网挂载能力。
  - 失败复查：strict validation 失败先修 spec/tasks 对应关系，不绕过校验。
  - evidence: vet 20260917T150435Z-921df8f2、CGO_ENABLED=0 build 20260917T150436Z-8b1c4857、strict validation 通过；docs/local-cas-mount.md 更新为实现行为、回退与回滚；不宣称 Server blob 表归属或公网挂载。

验证证据：`temp/integration-test-runs/20260917T150525Z-4de29166/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
