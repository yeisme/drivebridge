# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。根包只含设计与规格；代码任务归后续 owner change。

- [x] 1.1 依赖无；串行 A。核对 `backends.go` 现有后端、generic/OneDrive 能力差异、ReuseUpload 与版本钉住语义，写入基线。 | evidence: design.md 基线节；能力矩阵现状列。
- [x] 1.2 依赖 1.1；串行 A。冻结 A–F 六方案、平台卷/个人空间两角色、身份与版本钉住纪律、OpenList 信任边界。 | evidence: design.md；specs 六条 Requirement。
- [ ] 2.1 依赖 1.2；串行 A（实现波，cli/drivebridge owner change）。B 路径 runbook（rclone webdav→OpenList）落地 `cli/drivebridge/docs/`；doctor 增加 OpenList/WebDAV 可达提示。验收：真实自托管 OpenList 冒烟另行报告，不以 fixture 宣称。 **状态：2026-09-19 not-now**（一等网盘收口，见 `drivebridge-consumer-binding-v1`）；generic 逃生舱保留，本任务不再作为当前交付。
- [ ] 2.2 依赖 1.2；串行 A（实现波）。C gdrive 晋升：named 能力键 + md5 钉版 + copy 秒传测试。验收：`go test` 覆盖 md5 漂移拒绝与零字节 copy；`CGO_ENABLED=0` 构建保持。 **状态：改由 `drivebridge-consumer-binding-v1` Wave 1 handoff `drivebridge-first-class-backends-v1` 实施，本包不再单独排期。**
- [ ] 2.3 依赖 1.2；串行 A（实现波）。E OpenList REST provider（默认关闭）：能力键不出现即拒绝；临时 URL 纪律测试（不进输出/日志）。验收：关旗标下 `access_unsupported`；开旗标仅本地 fixture 验收。 **状态：2026-09-19 not-now**（与 2.1 同收口）。
- [x] 1.3 依赖 1.2；串行 V。根包 strict 校验与文档链接；不宣称运行时验收。 | evidence: `openspec validate drivebridge-storage-backends-v1 --strict --no-interactive` PASS；docs/README 已链。
