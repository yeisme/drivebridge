## Why

Agent 消费文件的方式是"问库"而不是"翻目录"：找素材要按名称/元数据检索；跟踪产出要"自上次以来变了什么"（轮询增量）；大量中间产物要按内容去重与保留策略管成本。这三件是 discovery 面，agent-workspace-v1 之后的下一层。

## What Changes（设计，实现归后续波）

- **search(space_id, query, [path])**：空间内名称（glob/子串）+ 元数据（MIME/大小区间/时长）检索；索引=state DB（File 表已存 name/mime/size/duration），陈旧度对账策略=检索后 stat 校验命中引用（沿用诚实就绪：索引未命中不代表远端没有）。
- **changes(space_id, since_cursor)**：游标增量订阅；cursor=state DB 单调序号（File/Operation Updated 已有基准），返回 新引用/版本变化/操作完成 三类事件；首次调用返回当前 cursor + 现状快照语义（不回放历史）。
- **内容寻址**：上传前查 File 表同 SHA-256 → 秒传（登记新 ref 复用远端对象，OneDrive 复制 item / generic 服务端 copy）；跨空间引用复用显式 `--link-ref`（登记引用，不复制字节）。
- **scratch/TTL 空间与 pin**：space 增 `ttl_days`（0=永久）；过期清理=手动 `gc` 命令扫描最后引用时间（不自动删除——对齐"不自动清理用户文件"纪律）；`pin(ref)` 提升 retention 标记。

## Capabilities

### New Capabilities
无（并入 drivebridge-local-files 增量需求，实现波落 delta）。

### Modified Capabilities
- `drivebridge-local-files`：Agent discovery 需求（search/changes/内容寻址/保留）——本波仅设计。

## Impact

只读检索+登记型去重+显式 gc：无删除、无覆盖、无自动清理；游标与 pin 是 state DB 新表。实现波分两批：search+changes 先行，内容寻址+TTL 其后。
