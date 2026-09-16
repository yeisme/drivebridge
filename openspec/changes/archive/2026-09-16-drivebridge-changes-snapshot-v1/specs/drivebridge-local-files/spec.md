## MODIFIED Requirements

### Requirement: Workspace directories and transfer CLI
工作区 SHALL 支持目录组织与直连 CLI：目录参数限定在空间钉定根内，传输与组织复用既有操作/恢复/引用合同；CLI 是用户本人终端的等价入口。同步等待传输的 CLI 命令 SHALL 在 `--events` 下流式输出 progress 事件（seq 单调、节流），人读模式在 stderr 刷新单行进度；`--agent` 投影 SHALL 将嵌套 map 扁平化一层并以计数表达切片。索引检索（find）与事件流（changes）SHALL 在排他进程锁被持有时回退只读快照并以 `read_only` 标注，快照命中保持 advisory。

#### Scenario: Organize and transfer by directory
- **WHEN** agent 或用户在空间内创建目录并指定目录上传、按目录列出
- **THEN** 目标名保持操作前缀（不替换既有文件），按路径列出的引用与根层引用同一稳定合同

#### Scenario: Path escape rejected
- **WHEN** 目录参数为绝对路径、含 .. 遍历或未清洗形式
- **THEN** 返回固定拒绝码，不触达空间外的远端路径

#### Scenario: mkdir idempotence
- **WHEN** 重复创建已存在的目录路径
- **THEN** 成功返回；同名文件占用该段时返回冲突而非覆盖

#### Scenario: CLI retry reuses the original operation
- **WHEN** 用户重复执行同一 CLI 传输命令而未显式提供幂等键
- **THEN** 确定性派生的键命中原 operation，恢复原传输而不创建重复文件

#### Scenario: Transfer progress is observable
- **WHEN** 上传或下载在 --events 下运行
- **THEN** stdout 流出 start→progress（bytes_transferred/bytes_total，节流）→end/error，seq 单调；人读模式进度只在 stderr 刷新

#### Scenario: Agent facts flatten nested payloads
- **WHEN** --agent 投影包含嵌套 map 或切片
- **THEN** 标量叶以点号键扁平化一层（fact.operation.status=…），切片以 _count 表达；多行与更深层留在 --json

#### Scenario: Index search survives the exclusive lock
- **WHEN** MCP 客户端持有进程锁时执行 find
- **THEN** 经只读快照按同语义过滤返回并以 read_only 标注；摘要明示命中仍需 stat 验证

#### Scenario: Event feed survives the exclusive lock
- **WHEN** MCP 客户端持有进程锁时执行 changes
- **THEN** 经只读快照按同游标语义返回（0=当前位置、ahead 拒绝）并以 read_only 标注
