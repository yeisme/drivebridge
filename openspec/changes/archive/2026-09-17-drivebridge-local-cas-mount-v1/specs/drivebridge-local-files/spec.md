## MODIFIED Requirements

### Requirement: Agent discovery surface

discovery SHALL 提供空间内检索、游标增量与内容寻址登记：检索命中后经 stat 校验再消费；游标只前进而首次订阅不回放历史；内容去重登记新引用而不覆盖远端对象。下载 SHALL 在本机目标路径或 CAS 缓存已有相同 SHA-256 且与固定 version 一致时跳过拉取。远程 Server 模式 SHALL 把秒传决策交给服务端 blob 命中，本机不得把他人摘要当作授权。

#### Scenario: Search with stale index honesty
- **WHEN** 检索命中本地索引但远端已变化
- **THEN** 消费前 stat 校验失败即报告版本变化；索引未命中不宣称远端不存在

#### Scenario: Cursor subscription
- **WHEN** agent 以游标轮询空间变化
- **THEN** 返回新增引用、版本变化与操作完成事件；游标单调且不回放首次订阅前的历史

#### Scenario: Content-addressed upload
- **WHEN** 上传内容的空间内同 SHA-256 已存在
- **THEN** 登记新引用复用既有对象（或显式链接引用）而不重传字节，不覆盖远端文件

#### Scenario: Content-addressed download
- **WHEN** 下载目标或 `.drivebridge-cas` 已有与固定 version 相同的 SHA-256
- **THEN** 操作完成且不从远端复制字节；version 变化时拒绝用旧缓存冒充
