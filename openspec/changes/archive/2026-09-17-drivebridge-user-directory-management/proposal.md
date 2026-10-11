## Why

用户目录缺少按状态、角色、团队和 Key 查询的真实数据合同，界面把不可见 Key 误显示为零，冻结及删除操作只能在混杂的大表单中寻找。需要提供明确的账号生命周期和对应权限验证。

## What Changes

- 新增分页用户目录、Key 元数据详情、账号操作影响与版本化写入接口。
- User 增加 archived、revision；冻结/解冻与删除归档/恢复通过治理服务执行。
- 复用现有身份验证、审计、请求失效和最后负责人约束，旧 account-status 同样检查新约束。

## Capabilities

### New Capabilities
- `account-directory`: 用户目录、账号管理及受限 Key 元数据。

## Impact

split-owner：本服务拥有身份、权限、GORM 与审计；client/drivebridge-client 提供筛选、详情和影响确认。字段和接口均增量兼容，既有登录 Key 明文仍只对本人开放。不包含永久删除文件、发送通知邮件或真实部署。
