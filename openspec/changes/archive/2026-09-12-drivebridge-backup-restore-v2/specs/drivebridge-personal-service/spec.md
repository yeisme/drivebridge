## ADDED Requirements

### Requirement: Backup verification and object manifest
备份 SHALL 附带对象清单并支持只读校验：清单与元数据交叉一致、抽样对象存在性可探测；校验失败按名报告，不宣称恢复就绪。

#### Scenario: Manifest matches metadata and objects
- **WHEN** 对一份备份执行校验且清单、元数据与抽样对象全部一致
- **THEN** 报告通过并给出抽样统计，恢复演练可继续

#### Scenario: Manifest or object drift detected
- **WHEN** 清单条目与元数据不一致或抽样对象缺失
- **THEN** 以固定错误码按名报告失败项，整体判定不可信，不给部分通过的恢复承诺

#### Scenario: Restore drill verification
- **WHEN** 在隔离目录完成恢复后执行只读校验
- **THEN** 校验覆盖备份三件一致性与清单交叉，密钥取回步骤包含在演练内
