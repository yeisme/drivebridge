## MODIFIED Requirements

### Requirement: Durable file transfer

系统 SHALL 保留稳定文件身份、不可变版本和原操作恢复。当授权范围内已存在相同明文 SHA-256 的 blob 时，系统 SHALL 发布新 version 指向该 blob，SHALL NOT 再写一份对象。未命中时 SHALL 保持原分片上传与复读校验。

#### Scenario: Interrupted upload
- **WHEN** 分片上传中断或服务重启
- **THEN** 查询原 operation 及已接收分片，校验相同源文件后继续，不创建重复文件

#### Scenario: Replayed complete
- **WHEN** 客户端重放已完成操作
- **THEN** 返回同一文件版本与回执

#### Scenario: Authorized blob reuse
- **WHEN** 同一 data owner 再次上传已存储的相同 SHA-256 到新路径
- **THEN** 操作完成且不写新对象，新旧 version 均可按原 URI 读取相同字节
