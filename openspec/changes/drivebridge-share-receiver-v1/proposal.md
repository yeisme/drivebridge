## Why

现有分享要求接收方每次持有完整链接秘密并重复提取码；打开事实只有匿名观测，无法支撑"接收方可验证"的交付分级。参考 AlterSend 的接收方设计（挑战证明、设备配对、会话上限），补齐无账号接收方的一次性验证、后续免密打开与有限额度合同，并与固定交付的接收事实分级衔接。

## What Changes

- 完善分享接收能力的加法合同：挑战应答打开（链接秘密不出现在请求体或 Cookie）、接收设备绑定与按设备撤销、打开/下载上限与单次分享、接收状态与 fixed-delivery 分级衔接。
- 准入 `fit`；唯一实现 owner 为 `cli/drivebridge/server`，本文不把任务迁入根仓或复制到其他 surface。
- 保留旧 `POST /api/v1/shared/{id}/open`、既有 Cookie、URI、错误码与 audience 语义；所有新增能力为 required、分阶段待实施。
- 本次只交付规格与文档；不实现产品代码、不修改真实配置、不接真实云账号或调用付费模型。

## Capabilities

### New Capabilities
- `shares`（增量）：挑战应答打开、不落地秘密的会话、接收设备绑定、分享上限与接收状态衔接的正常、失败、权限、并发与恢复合同。

### Modified Capabilities
无既有规格的破坏性替换；旧行为兼容测试作为本包实施门。

## Impact

- owner `cli/drivebridge/server` 的应用合同及其文档；接口映射见 [设计](design.md)，原子任务见 [任务](tasks.md)。
- 跨 owner 范围与保留能力见 [根 handoff](../drivebridge-file-platform-evolution-v2/handoffs.md)。根包只维护设计关系，不记录本包实现完成状态。
- 规划 HTTP/SDK、可选配置和数据模型均为加法；未启用时旧系统可运行，主程序保持 pure Go。
