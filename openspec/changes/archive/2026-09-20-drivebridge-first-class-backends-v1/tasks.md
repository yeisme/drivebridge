## 1. 目录合同

- [x] 1.1 依赖无；串行 A。`KindDef` 加法 `tier`/`bind_default`；赋值两层一等与逃生舱；导出 `BindDefaultKinds`。验收：catalog 单测。 | evidence: storagekinds.go KindDef/BindDefaultKinds。
- [x] 1.2 依赖 1.1；串行 A。更新 `TestStorageCatalogStaysHonest`，新增默认发现单测：非 onedrive 保持 generic 真值，local/s3/gdrive 仅产品发现一等。验收：`go test ./internal/bridge -run 'StorageCatalog|Kinds|FirstClass|EnsureAndProbe' -count=1`。 | evidence: 该命令 exit 0。

## 2. 投影与文档

- [x] 2.1 依赖 1.1；串行 A。MCP `capabilities` 加法 `bind_default_kinds`；`storage kinds` 信封投影 `tier`/`bind_default`/`bind_default_kinds`。验收：CLI storage 测试。 | evidence: `CGO_ENABLED=0 go test ./cmd/drivebridge -count=1 -run 'TestCLI/storage'` exit 0。
- [x] 2.2 依赖 2.1；串行 D。`docs/storage-recipes.md` 写明两层一等，并声明 local 是可管理默认存储、不是新网盘厂商。

## 3. 校验

- [x] 3.1 依赖 1.2、2.2；串行 V。聚焦测试与 `openspec validate drivebridge-first-class-backends-v1 --strict --no-interactive`。 | evidence: strict PASS；bridge 聚焦测试 exit 0。
