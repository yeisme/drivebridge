## Why

归档管道只写服务端受管 S3 存储；此前非受管目标空间要到 Complete 阶段才以 storage_disconnected 失败——此时字节已流过、配额已预留，错误码也与"存储类型不支持"的实情不符。client 已有 unsupported 文案但后端从不返回该码。

## What Changes

`ArchiveVersion` 入口增加受管目标校验：目标空间缺失返回 not_found；其连接缺失、禁用或非 s3 类别返回 unsupported。校验发生在任何 staging、配额预留与字节传输之前；resume 路径复用同一校验（目标失去受管存储后恢复被如实拒绝）。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-service`：Managed version archive 需求增加非受管目标早拒绝场景。

## Impact

backend-server/drivebridge 全量实现与验收；client 文案已就位，无需改动；不改任何既有错误码语义。
