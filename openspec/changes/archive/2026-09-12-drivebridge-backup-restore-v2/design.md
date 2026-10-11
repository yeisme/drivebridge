## Context

`backup` 现复制 `metadata.sqlite`+`instance.id`（0600），`instance.key` 明确排除、独立秘密渠道；doctor 已能校验三件一致性（deployment-readiness）。对象 key 按实例/用户/空间/版本生成（contract-map.md），版本表有明文 SHA-256。

## Decisions

- **manifest.json**（备份目录新文件，0600）：`{schema_version, instance_id, created_at, counts}` + 条目 `[{version_id, object_key, sha256_plaintext, size}]`；从 GORM 只读导出（含回收站内软删文件的现行版本——恢复后语义一致）。与 metadata.sqlite 同次备份生成，原子性=同目录写入+fsync。
- **`backup --verify`**：三段校验——(1) 清单 vs 元数据重新查询（计数+逐条 version_id/摘要一致）；(2) 抽样对象存在性（`HeadObject`，默认抽 N=min(50, 全量)，`--verify-sample` 可调，凭据复用平台连接加密凭据，不外泄）；(3) doctor 式一致性报告（密钥缺失提醒、manifest schema 版本）。输出走 CLI 输出合同（`--json/--agent`）；失败项固定码（`manifest_mismatch`、`object_missing`、`sample_unverifiable`）。
- **doctor 集成**：`doctor --backup <dir>` 只读校验既有备份目录（三件+manifest 交叉），供恢复演练后验证。
- **演练 runbook**（deployment.md §5 增强）：备份→在隔离 state 目录恢复（含原 instance.key）→doctor --backup 全绿→起 serve 不带 --enabled 冒烟→清理；每步可勾选，证据落 temp/integration-test-runs。
- **密钥托管**：写明单人场景指引——instance.key 与备份分离存放（密码管理器/离线介质），恢复演练必须包含"从托管渠道取回密钥"一步；不引入双人门（已砍）。
- **元数据备份加密选项**：`backup --encrypt`（与 content-encryption-v1 共用实例密钥派生流， AES-GCM 包 tarball）——依赖该 change 的密钥派生设计，实现波按其落地顺序排。

## Interfaces

`cmd/drivebridge-server/`（backup --verify/--encrypt、doctor --backup）、`internal/service/backup.go`（manifest 导出+校验）、`docs/deployment.md`。

## Verification / Rollback

实现波验证：fixture state 备份→manifest 交叉全绿；篡改 manifest/object 造三段失败各一；doctor --backup 两态；恢复演练脚本化冒烟。回滚：不传新旗标=旧行为逐字节不变。

## 去向

实现波：manifest+verify+doctor --backup+runbook；`--encrypt` 排在 content-encryption-v1 之后。
