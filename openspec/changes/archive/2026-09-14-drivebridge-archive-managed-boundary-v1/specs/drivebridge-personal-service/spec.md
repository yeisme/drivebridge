## MODIFIED Requirements

### Requirement: Managed version archive
系统 SHALL 支持把指定固定版本经服务端复制进受管目标空间：以幂等键去重，绝不覆盖既有文件，源版本与可选期望头版本冲突即拒绝；归档作为可恢复操作保留来源与 provenance。目标空间缺乏可用受管 S3 存储时，系统 SHALL 在任何字节流动、配额预留或操作创建之前返回 unsupported。

#### Scenario: Never replaces an existing file
- **WHEN** 目标路径已存在未回收文件
- **THEN** 归档落在自动改名的新路径（原名加序号），既有文件保持不变

#### Scenario: Pinned and conflict-checked source
- **WHEN** 指定版本不属于源文件，或期望头版本与源当前头不一致
- **THEN** 返回 not_found 或 version_conflict，不产生任何传输

#### Scenario: Idempotent replay and resume
- **WHEN** 同一归档请求以相同幂等键重放，或归档中断后恢复
- **THEN** 幂等命中返回既有操作；恢复按记录的源固定版本重新传输字节，不生成新内容

#### Scenario: Non-managed destination rejected before transfer
- **WHEN** 目标空间的存储不是启用中的 S3 连接（缺失、禁用或非 s3 类别）
- **THEN** 返回 unsupported，不创建操作、不预留配额、不传输任何字节
