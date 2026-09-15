# Design: drivebridge-find-snapshot-v1

## 决策

1. **GORM ro 模型直查而非手写 SQL**：snapshotSearch 用 bridge.File 模型复用过滤/排序/投影语义，规避 GORM 列名≠json 名的物理列陷阱（本仓第三次踩后固化此模式）。
2. **advisory 语义显式化**：索引可能落后远端；快照路径的摘要与 read_only 标注都提醒 stat 验证——search 本来的纪律（hits are unverified）不因快照而弱化。
3. **不做 ls/stat/changes 的快照**：它们的语义含远端验证或 provider 交互，快照会冒充验证；留给 OpenReadonly 完整构造器的后续设计。
