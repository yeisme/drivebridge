# drivebridge-find-snapshot-v1

find 在排他锁被持有时回退只读索引快照（与 recent/operation 同纪律）；快照命中保持 advisory，stat 仍是验证者
