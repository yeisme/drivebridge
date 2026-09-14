## Context

rclone bisync 库面：`cmd/bisync.Options{Resync, ResyncMode(Prefer), MaxDelete, CreateEmptySrcDirs, ...}` + run 入口；journal 文件记录两侧快照，异常（journal 缺失/单侧大变）默认拒绝执行。DriveBridge 已有 Operation 模型（中断标 `interrupted`、恢复用原 operation）与不可变版本语义。

## Decisions

- **同步空间**：现有 Space 即同步单元（local root ⇄ 远端根），不建第二套绑定；`sync` 以 `--space <id>` 触发。generic 与 onedrive 空间都可同步（引擎只看 fs 两端）。
- **Operation 集成**：每次 sync 建 Operation（kind=`sync`，Status queued→transferring→completed/failed/interrupted），进程重启把进行中 sync 标 `interrupted`（沿用现行迁移）；恢复=查询原 operation 后重跑（bisync journal 保证两侧一致才继续，journal 损坏则要求显式 resync）。**同步永不触发生成/交接**，与 save_output 同一纪律。
- **守卫**：首跑（无 journal）强制要求 `--resync` 显式确认（resync 语义=path1 本地优先？否——`ResyncMode` 固定 `path1`，即以本地为基准首同步，文档明示）；`MaxDelete` 默认收紧到 10（bisync 默认 50，个人文件场景从紧），可显式覆盖但下限 1；`--dry-run` 输出计划（新增/更新/删除/冲突四类计数+明细走 CLI 输出合同 `--json/--agent`）。
- **冲突策略**：keep-both——两侧同名同改时保留两份，远端新副本命名 `<name>.conflict-<operation-id>`（不覆盖用户选定文件，与 upload 命名纪律一致）；本地侧冲突副本命名 `<name>.conflict-<operation-id>`。无 last-writer-win，不静默择优。
- **删除语义**：bisync 的删除即删除（远端删除=移出空间，不进 DriveBridge 回收站概念——本仓无回收站；被删文件的既有稳定引用失效为 `reference_not_found`，stat 如实报告）。MaxDelete 守卫先于执行。
- **软链接**：完全沿用 generic-backends-v1 策略（默认拒绝、opt-in 物化），sync 扫描前置检查。
- **调度**：cron/常驻延期；首版仅手动 CLI + `sync --status`（查原 operation）。MCP 面不加 sync 工具（同步是操作者动作，不是 Agent 动作；防 Agent 自动改动用户本地目录树）。
- **报告与证据**：dry-run 与实跑摘要入 `temp/integration-test-runs/`（经 check.py），明细不含 URL/token。

## Interfaces

`internal/bridge/sync.go`（engine 包装：Options 构造、journal 目录放 state 下 0700、Operation 集成）、`cmd/drivebridge`（`sync` 子命令 + `--space/--resync/--dry-run/--max-delete`）。spec：新 capability `drivebridge-sync`。

## Verification / Rollback

实现波验证：local-backend 双临时目录 fixture 全矩阵（首跑守卫、正常双改、冲突 keep-both、单侧删除+MaxDelete 触发、中断→journal 恢复、journal 损坏→要求 resync、dry-run 报告）+ race。回滚：不调用 sync 即无影响；journal 目录可整删。

## 去向

实现波：sync engine + 命令面 + 全矩阵测试；后续波（可选）：调度与通知。
