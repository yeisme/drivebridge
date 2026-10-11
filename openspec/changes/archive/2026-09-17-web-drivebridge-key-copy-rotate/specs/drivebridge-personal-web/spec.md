## MODIFIED Requirements

### Requirement: Admin key-user management page
用户 SHALL 在网页端查看本人 Key 元数据、复制真实 Key、再次复制、遮罩查看与确认轮转。admin 的既有批量签发及吊销按服务端权限提供，管理身份 SHALL NOT 获得他人 Key 的取回能力。输入框和下拉框 SHALL 具有一致的绿色主题、聚焦与禁用样式，并支持键盘与窄屏。

#### Scenario: Admin issues users
- **WHEN** admin 提交批量签发表单
- **THEN** 明文密钥在签发弹窗展示并可复制，列表即时更新；关闭后仅所属用户可从本人 Key 页面取回

#### Scenario: One-time display
- **WHEN** 签发弹窗关闭后再次进入管理页
- **THEN** 列表只有生命周期元数据，无任何明文或哈希

#### Scenario: Revoke with confirmation
- **WHEN** admin 吊销某密钥并确认
- **THEN** 该行翻转为已吊销状态

#### Scenario: Hidden from non-admin
- **WHEN** 普通用户登录
- **THEN** 用户可进入本人 Key 页面，不出现管理员签发和跨用户管理动作

#### Scenario: Repeat copy and rotation
- **WHEN** 用户复制本人有效 Key 或确认轮转
- **THEN** 复制内容为实际密钥，支持再次复制；轮转显示旧 Key 立即失效的影响，成功后可查看和复制新 Key

#### Scenario: Clipboard unavailable
- **WHEN** 浏览器拒绝自动复制
- **THEN** 显示默认遮罩的可手动选择密钥面板；关闭面板后清除明文状态

