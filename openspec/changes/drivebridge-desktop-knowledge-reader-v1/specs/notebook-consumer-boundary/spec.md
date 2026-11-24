## ADDED Requirements

### Requirement: 客户端消费合同由 DriveBridge 独立验收

DriveBridge SHALL 在本 owner 维护 source-first 编辑工作台、Electron main/preload、renderer DTO、opaque recovery slot、managed guard 和三平台验收。Pinax SHALL 只负责其公开 source/writer/sync/attachment/recovery 合同。消费者未通过 SHALL NOT 阻止 Pinax CLI 工程收口。

#### Scenario: 接收原 Pinax 中的消费者任务

- **WHEN** `pinax-drivebridge-workbench-v1` 从 Pinax 迁至本 owner 历史 handoff
- **THEN** D03/D08、D14、D09/D10 继续使用本 change 原 canonical 状态，历史包不创建第二执行队列，pending 项不因迁移而通过

### Requirement: 闭合桌面 DTO 与恢复边界

Electron main/preload SHALL 持有 loopback bearer、session 和原生选择票据。renderer SHALL 只消费 opaque identity、revision、有界 projection 和原 recovery key，不接收凭据、SQLite、`.pinax` 路径、任意文件系统写指令或 provider payload。

#### Scenario: 草稿或回执未决

- **WHEN** 编辑产生草稿或 mutation 回执丢失
- **THEN** 正文仅在内存中存在，恢复槽只保存原 opaque key，使用 owner 的 `operation.show`/`operation.reconcile`，不新建重试 identity

### Requirement: 源码与嵌入内容分开编辑

DriveBridge SHALL 只将 rich/composed 文档作为固定 revision 预览，单独打开并授权嵌入目标。编辑 SHALL 调用 Pinax 原 Markdown source 的 proposal/确认/apply/receipt 合同，不将拼接结果写回父文档。附件 SHALL 使用 Pinax bounded input 合同。

#### Scenario: 嵌入来源不可用

- **WHEN** 嵌入来源不可用、歧义、循环、超深度或未授权
- **THEN** 展示明确状态，不提交父文档写入；当前 proposal-only 纵切不据此升级为 apply 已验收
