# 分享接收方：挑战应答打开、设备绑定与额度

状态：设计完成候选；实现、本地运行验证、真实后端验证、发布部署均未由本包证明。

- [提案](proposal.md) · [设计与接口](design.md) · [任务](tasks.md) · [能力规格](specs/shares/spec.md)
- 设计参照 [AlterSend](https://github.com/denislupookov/altersend) 接收方机制（挑战证明、设备配对、会话上限），采纳范围与不采纳理由见 design 对照表
- [根能力账本与owner交接](../drivebridge-file-platform-evolution-v2/handoffs.md)（工作区导航；独立checkout实施以本地文件为准）

在本 owner 目录验证：`openspec validate drivebridge-share-receiver-v1 --strict --no-interactive`。
