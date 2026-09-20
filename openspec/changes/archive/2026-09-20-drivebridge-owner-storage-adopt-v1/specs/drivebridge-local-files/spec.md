## ADDED Requirements

### Requirement: 已 adopt 空间的写入围栏

在已 adopt 的空间内，变异文件操作（upload、delete/trash、mkdir、move、rename）若相对路径属于 owner 保护路径，SHALL 返回 `owner_protected` 且不改写该路径。保护路径包括：`.pinax` 与 `.pinax/**`；`.scaena` 元数据（SHALL NOT 包括 `.scaena/objects/sha256/**` 下的 CAS 对象字节）；SQLite/WAL（`*.sqlite`、`*.sqlite-wal`、`*.sqlite-shm`、`*.db` 及对应 wal/shm）；token 文件（basename `token` 或 `*.token`）。上述路径的 list/stat MAY 仍然允许。未 adopt 空间 SHALL NOT 因本要求改变行为。

#### Scenario: 拒绝改写 .pinax
- **WHEN** 对已 adopt 空间请求 mkdir 或 upload 到 `.pinax` 或 `.pinax/config.yaml`
- **THEN** 返回 `owner_protected`，本地/远端该文件不变

#### Scenario: 允许浏览保护路径
- **WHEN** 对已 adopt 空间 list 或 stat `.pinax` 或 `.scaena/objects/sha256/...`
- **THEN** 操作不被 `owner_protected` 拒绝

#### Scenario: CAS 对象字节可 transfer
- **WHEN** 变异路径为 `.scaena/objects/sha256/<…>` 下的对象字节
- **THEN** 不因 `.scaena` 元数据规则返回 `owner_protected`
