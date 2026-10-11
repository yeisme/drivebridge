## Why

承接用户批准的语义驱动视频生产与 SoloCut v2.2 计划，在 DriveBridge 现有能力上补齐可消费合同和本仓验证，避免将演示结果当作正式服务。

## What Changes

- 补齐素材进入制作与成片回存的版本、摘要、受控传输和回执说明。引用不是远端可读路径；上传、生成和采用分别记录。版本漂移后重新准备，unknown 恢复原传输。
- 复用 稳定文件引用、prepare_reference、save_output 与原 transfer 恢复，通过固定引用与公开回执交接。
- 增量兼容、原任务恢复与本仓验收；实现状态以 tasks.md 为准。

## Capabilities

### New Capabilities
- `semantic-video-file-handoff`: file ref/version/digest、transfer identity、permission/source facts 与 handoff receipt。

### Modified Capabilities

无旧 wire 语义重解释。

## Impact

owner=DriveBridge，本域能力为 fit，下游消费为 split-owner。不复制生成任务或成片接受，不泄漏临时访问地址、宿主路径或凭据。 根仅维护交接；本仓任务不以其他仓库实施、付费调用或发布为归档门。


## 本轮增量设计

继续细化生产链路的固定来源、当前权限、版本映射、原任务恢复、公开消费与兼容验收，范围见 [design](design.md)。实施由新增 4.x 未完成任务追踪；既有完成项及证据保留。此次仅完善设计、规格和任务，不宣称新增 runtime 已接通，不运行真实模型或变更生产配置。
