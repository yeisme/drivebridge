# profile-api Specification

## Purpose
定义客户端对本人资料接口的增量消费：保留稳定用户标识，展示并修改服务端允许的昵称和头像引用。兼容旧响应，头像选择遵守文件归属和授权边界。

## Requirements

### Requirement: 兼容身份字段
客户端 MUST 保留 user.id 与 user.name，新增 display_name 和 avatar_ref 为可选展示字段。

#### Scenario: 旧服务响应
- **WHEN** 服务端未提供展示名称或头像
- **THEN** 页面使用原有 name 和首字头像

### Requirement: 保存本人资料
设置页 MUST 通过本人会话提交资料，失败时保留编辑内容。

#### Scenario: 保存与重新读取
- **WHEN** 用户提交有效资料并在另一会话读取
- **THEN** 显示相同资料，登录身份与权限不改变

### Requirement: 受控头像选择
设置页 MUST 从服务端返回的本人图片候选选择固定版本，不接受任意远程链接。

#### Scenario: 头像不可用
- **WHEN** 图片已删除或读取被拒绝
- **THEN** 回退到首字头像，不扩大文件权限
