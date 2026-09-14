## ADDED Requirements

### Requirement: Guarded bidirectional sync
sync SHALL 以显式守卫执行双向同步：首跑必须 resync 确认、删除比例受收紧上限约束、dry-run 可先行预览；不得由 MCP 或 Agent 自动触发。

#### Scenario: First run without resync confirmation
- **WHEN** 同步空间没有既有 journal 而未显式要求 resync
- **THEN** 拒绝执行并说明需要确认，不猜测基准侧

#### Scenario: Deletion exceeds the tightened cap
- **WHEN** 单次同步将删除的条目比例超过收紧上限
- **THEN** 中止执行并报告比例与上限，需显式覆盖才可继续

#### Scenario: Dry-run preview
- **WHEN** 以 dry-run 运行
- **THEN** 输出新增/更新/删除/冲突四类计划与计数，不改动任何一侧

### Requirement: Keep-both conflict and honest recovery
同步 SHALL 对两侧同变冲突保留双方副本并以操作 ID 命名，不得静默择优；中断后经原 operation 与 journal 恢复，journal 不可信时要求显式 resync。

#### Scenario: Both sides changed the same path
- **WHEN** 同一相对路径在两侧都发生修改
- **THEN** 双方副本均保留并带冲突标识命名，报告列出冲突项

#### Scenario: Interrupted sync recovered
- **WHEN** 同步进程中断后重新查询并重跑
- **THEN** 经原 operation 恢复执行；journal 损坏或两侧快照不可信时拒绝并要求显式 resync，不盲目重传

#### Scenario: Deleted remote file reference
- **WHEN** 已删除远端文件仍被既有稳定引用查询
- **THEN** 返回固定 not found 错误，不虚构存在也不触发恢复生成
