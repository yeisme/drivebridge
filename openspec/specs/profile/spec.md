# profile Specification

## Purpose
TBD - created by archiving change drivebridge-profile-v1. Update Purpose after archive.

## Requirements

### Requirement: 兼容资料更新
服务 MUST 保留 id/name 并允许本人浏览器会话更新展示名称。

#### Scenario: 显示名称持久化
- **WHEN** 有效会话提交 1–80 字符展示名称
- **THEN** 保存并返回资料，另一会话读取相同字段

#### Scenario: 校验失败
- **WHEN** 名称为空、超长、包含控制字符，或缺少 CSRF
- **THEN** 拒绝请求并保留旧资料

### Requirement: 受控头像
服务 MUST 验证头像固定版本属于本人个人网盘，不允许远程 URL 或其他用户资源。

#### Scenario: 非本人图片
- **WHEN** 用户提交他人图片或不存在的引用
- **THEN** 整个更新失败，展示名称也不会部分更新

#### Scenario: 清空与撤销
- **WHEN** 提交 null 头像或图片后来被回收
- **THEN** 清空不删除源文件，读取撤销的文件不会返回图片内容
