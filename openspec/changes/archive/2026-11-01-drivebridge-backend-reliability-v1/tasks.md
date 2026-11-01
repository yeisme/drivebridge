# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。

- [x] 1.1 为 BlobGC 增加 claim token 与时间字段并完成可重复迁移 | evidence: server/internal/service/model.go 与 blobs.go 已加入 GCClaimedAt/GCClaimToken，AutoMigrate 重复启动通过。
- [x] 1.2 实现启动后的 BlobGC abandoned claim reconcile 与 token 条件删除 | evidence: reconcileBlobGC 在持锁启动后清理 abandoned claims；BlobGC 删除/释放均按 token 条件更新。
- [x] 1.3 补充 BlobGC 崩溃、dry-run、存储失败和引用竞争测试。 | evidence: TestBlobGCClaimRecoversAfterRestart 与 Blob focused suite 通过，SQLite evidence 20261026T021718Z-9db8b5d4，PostgreSQL evidence 20261026T021733Z-25468f09。
- [x] 2.1 扩展 metadata JSONL 导出导入的 workspace 状态集合 | evidence: metadata JSONL 新增 workspace grants/groups/organizations/requests/submissions/decisions/credentials/suggestions/invocations 导出导入。
- [x] 2.2 为迁移快照增加确定性 fingerprint 并保留旧 count 摘要 | evidence: metadataFingerprints 加入确定性 SHA-256 指纹，保留 imported/source/target count 摘要与兼容输出。
- [x] 2.3 补充相同数量状态漂移、空表导入和事务回滚测试 | evidence: TestFPTeamRuntimeMetadata 与 Blob metadata 回归通过；空 workspace slice 不执行空插入、缺失目标 workspace schema 明确拒绝非空数据、导入事务保持原子；SQLite 20261026T021718Z-9db8b5d4。
- [x] 3.1 增加 SQLite 与显式 PostgreSQL fixture/database mode 入口 | evidence: fixtures 支持 DRIVEBRIDGE_TEST_DATABASE=postgres/DRIVEBRIDGE_TEST_PG_DSN；每个临时 state 使用独立 PostgreSQL schema。
- [x] 3.2 运行 BlobGC 与 metadata focused suites 的双库验证并写入六件套证据 | evidence: make test-backend 通过；SQLite 20261026T021718Z-9db8b5d4，PostgreSQL 20261026T021733Z-25468f09。
- [x] 4.1 更新服务端迁移命令、后端文档和恢复边界 | evidence: docs/delivery/backend-foundation-progress.md 与 docs/server/file-platform/runtime-status.md 已记录 BlobGC recovery、workspace metadata fingerprints 与双库命令。
- [x] 4.2 执行 openspec strict、go vet、pure-Go build 与 diff check | evidence: openspec strict、go vet ./...、CGO_ENABLED=0 go build ./cmd/drivebridge-server 与 git diff --check 通过。全量 server test 证据 20261026T020221Z-d64f263f；最新变更另经双库 focused 复验。
