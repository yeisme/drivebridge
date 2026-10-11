## 1. 实施与验证

- [x] 1.1 GORM 新增 `blobs` 表（`sha256`/`size`/`refs`/`object_key`/`encrypted`/`space_id`，加密空间主键维度叠加 `space_id`）与 `Version.blob_sha256` 可空列；旧行回填前按自身 object_key 读 | owner: `internal/service/model.go`、`internal/service/storage.go` | 依赖： 无；Lane A | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： 迁移幂等，迁移后旧 version 仍按旧键可读；失败复查： 用空库与既有 fixture 库各跑一次迁移。
- [x] 1.2 对象键增量双写： 明文 `<instance>/blobs/<sha256>`，加密 `<instance>/blobs/<space>/<sha256>`，旧 `<instance>/<owner>/<space>/<version>` 保留至 version 迁移或 purge | owner: `internal/service/files.go`、`internal/service/storage.go` | 依赖： 1.1；Lane A | 验证： 同上命令覆盖新写路径；预期： Complete 后 blob 键存在且旧 URI 读取字节不变；失败复查： 对照 design.md 键模板，区分明文/加密两条路径。

## 2. 实施与验证

- [x] 2.1 create_upload 秒传命中： 当前 Principal 对目标空间可写、blob 属于同一 `dataOwner()`、加密空间要求同一 `space_id`；命中即 completed、`bytes_transferred=0`、创建可查询 operation；仍校验配额与幂等指纹；配额预留→立即完成→`reserved` 回退、`used` 增加逻辑 size | owner: `internal/service/files.go`、`internal/service/lifecycle.go` | 依赖： 1.2；Lane B | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： 秒传零字节完成、`used_bytes` 增加且 `reserved_bytes` 无残留，他人空间同摘要不命中；失败复查： 核对 `dataOwner()` 取值与配额状态机。
- [x] 2.2 Complete 写入 blob 键与并发去重： 同一 sha256 的 Complete 用事务占 blob 行，后到者改引用并删除自己刚 Put 的重复对象；purge 减引用，计数到 0 不自动删对象 | owner: `internal/service/files.go`、`internal/service/storage.go` | 依赖： 2.1；Lane B | 验证： `python3 scripts/check.py -- go test -race ./internal/service -run 'Blob|Complete' -count=1`；预期： 并发双 Complete 只余一份对象、引用计数为 2，无重复键残留；失败复查： race 报告先定位事务边界，再核对象删除补偿路径。
- [x] 2.3 漂移与失败语义： 元数据有 blob 但存储读回失败返回 `blob_unavailable`，不发布空文件、不把命中当成功；越权 `GET /api/v1/blobs/{sha256}` 返回 `not_found` 不返回 ref_count | owner: `internal/service/files.go`、`internal/service/http.go`、`internal/service/error_messages.go` | 依赖： 2.1；Lane B（与 2.2 并行） | 验证： 同 2.1 命令覆盖对象缺失与越权用例；预期： 错误码精确、无内容泄露；失败复查： 确认不走 latest/空响应降级。

## 3. 实施与验证

- [x] 3.1 HTTP 增量： 成功体可选 `blob_reused`，新增 `GET /api/v1/blobs/{sha256}`（至少一个 version 有读权才返回计数）；不新增 `POST /api/v1/mounts`、无匿名 blob GET；Range 读取继续走 `GET /api/v1/files/{id}/content` 返回 206 | owner: `internal/service/http.go`、`internal/service/files.go` | 依赖： 2.x；Lane C | 验证： `python3 scripts/check.py -- go test ./internal/service -count=1`；预期： 旧必填项、旧路径、旧错误码不变，Range 不暴露 blob 键；失败复查： 对照 docs/contract-map.md 旧合同回归。
- [x] 3.2 SDK 与 MCP 增量： SDK 暴露可选 `blob_reused`；MCP `create_upload` 命中即 completed，不新增必填参数，输出不出现临时 URL | owner: `sdk`、`internal/service/mcp.go` | 依赖： 3.1；Lane C（与 3.3 并行） | 验证： `python3 scripts/check.py -- go test ./sdk -count=1` 及 `./internal/service -run MCP`；预期： SDK/MCP 旧调用形状兼容；失败复查： 检查输出脱敏与可选字段默认值。
- [x] 3.3 owner CLI `drivebridge-server blob stat` / `blob gc --dry-run`： gc 默认只报告可回收项（refs=0），未带确认不删对象 | owner: `cmd/drivebridge-server/commands.go`、`cmd/drivebridge-server/root.go` | 依赖： 2.2；Lane C（与 3.2 并行） | 验证： `python3 scripts/check.py -- go test ./cmd/drivebridge-server -count=1`；预期： dry-run 零删除、输出为安全元数据；失败复查： 用真实 CLI 进程复跑，核对 exit code。
- [x] 3.4 备份清单 blob 交叉： 备份/恢复清单记录 blob 引用，恢复后 blob 计数与对象一致 | owner: `internal/service/backup.go` | 依赖： 2.2；Lane C（与 3.2/3.3 并行） | 验证： `python3 scripts/check.py -- go test ./internal/service -run 'Backup' -count=1`；预期： 备份→恢复后共享 blob 的 version 均可读；失败复查： 对照既有备份清单格式做兼容检查。

## 4. 实施与验证

- [x] 4.1 race 全量与失败用例矩阵： 同用户秒传、越权不泄露、加密跨空间拒绝、幂等冲突、并发双 Complete、blob 对象缺失 → `blob_unavailable` | owner: `internal/service`、`sdk` | 依赖： 1.x–3.x；Lane V | 验证： `python3 scripts/check.py -- go test -race ./internal/service ./sdk -count=1`；预期： 全绿且测试输出确实包含上述新用例（拒绝 no tests to run）；失败复查： 归因 introduced/pre-existing/environmental 后回对应 lane。
- [x] 4.2 收尾验证与证据归档： 运行 `python3 scripts/check.py`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server`、`openspec validate drivebridge-cas-blob-store-v1 --strict --no-interactive`；证据写入 `temp/integration-test-runs/<run-id>/` 六件套并脱敏；同步 `docs/intranet-cas.md` 与 `docs/contract-map.md`；全部通过后 `openspec archive` | owner: 全项目 | 依赖： 4.1；Lane D | 预期： 四命令全绿、strict 校验通过；失败复查： 修源因后整条重跑，不以跳过用例清绿。

验证证据：`temp/integration-test-runs/20260917T152017Z-5da627fd/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
