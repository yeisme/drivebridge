# 历史交接：Pinax writer/sync 与 DriveBridge 客户端

2026-11-03 按用户要求从 `cli/pinax/openspec/changes/` 无损迁回消费者 owner。本包完整保留迁移时的设计、任务与证据，**不作为第二套 active 执行队列**。原名 Workbench 仅保留历史，不恢复独立 Workbench 产品。

消费者唯一任务源是 [desktop knowledge reader tasks](../../changes/drivebridge-desktop-knowledge-reader-v1/tasks.md)：

| 原任务 | canonical task | 当前解释 |
| --- | --- | --- |
| D14.1 | D14.1 | 同源 writer/sync DTO；保留既有完成证据 |
| D14.2 | D14.2.1、D14.2.2 | 纯状态机与 opaque 槽已有组件证据；D14.2 父项仍未完成 |
| D14.2a | D14.2a | 只读 sync 面板组件；不表示远端/写 admission 通过 |
| D14.3 | D14.3 | 已有 proposal-only Electron 纵切；不代表 apply 或完整 reader |
| D03/D08 | D03、D03.5、D03.6、D08、D08.1 | guard、旧 binary、跨 state、会话与迁移验收只由 DriveBridge 推进 |
| 三平台、安装/签名、reader 完整旅程 | D09、D10.mac、D10.win、D10.linux、D11 | 保留原 pending，Pinax 代码完成不替代 |

Pinax 自己的 R02/R03 已通过任务 CLI 接收到 `cli/pinax/openspec/changes/pinax-release-qualification-v1/`，旧 tasks 中只留迁出引用。Pinax 已实现的 P01–P05 和工程证据作为本包历史保留，当前工程收口见 Pinax `pinax-sync-attachment-contract-closeout-v1`。双方只消费公开合同，不共用数据库或凭据。

后续只在 canonical change 更新状态。本包历史 checkbox 不参与任何一方当前未完成任务计数，也不能据此宣称消费者验收通过。
