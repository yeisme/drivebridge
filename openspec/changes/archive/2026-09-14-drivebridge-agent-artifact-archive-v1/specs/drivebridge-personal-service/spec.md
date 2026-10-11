## ADDED Requirements

### Requirement: Scoped operation inbox
系统 SHALL 提供收件箱查询：列出调用者近 30 天自有操作，支持 status/caller/space 可选过滤，且仅返回当前身份对操作目标具备读权限的条目；数据库不可用时如实失败。

#### Scenario: Permission filtered after limit
- **WHEN** 操作归属调用者但当前身份已失去对目标空间或路径的读权限
- **THEN** 该条目不出现在结果中，返回数量可以少于请求的 limit

#### Scenario: Honest failure
- **WHEN** 状态数据库查询失败
- **THEN** 返回 database_unavailable，不以空列表冒充成功

### Requirement: Managed version archive
系统 SHALL 支持把指定固定版本经服务端复制进受管目标空间：以幂等键去重，绝不覆盖既有文件，源版本与可选期望头版本冲突即拒绝；归档作为可恢复操作保留来源与 provenance。

#### Scenario: Never replaces an existing file
- **WHEN** 目标路径已存在未回收文件
- **THEN** 归档落在自动改名的新路径（原名加序号），既有文件保持不变

#### Scenario: Pinned and conflict-checked source
- **WHEN** 指定版本不属于源文件，或期望头版本与源当前头不一致
- **THEN** 返回 not_found 或 version_conflict，不产生任何传输

#### Scenario: Idempotent replay and resume
- **WHEN** 同一归档请求以相同幂等键重放，或归档中断后恢复
- **THEN** 幂等命中返回既有操作；恢复按记录的源固定版本重新传输字节，不生成新内容
