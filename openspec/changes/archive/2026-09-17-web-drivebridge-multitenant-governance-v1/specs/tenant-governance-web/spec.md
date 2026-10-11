## ADDED Requirements

### Requirement: Tenant-aware workspace
Web SHALL 按租户、团队和空间权限展示页面，管理访问默认只读，所有写入由服务端授权。

#### Scenario: Tenant switch during an in-flight request
- **WHEN** 用户切换租户或空间
- **THEN** 清空旧视图并拒绝旧上下文响应，取消在途传输

### Requirement: Approval and notification center
Web SHALL 提供申请、审批、明确执行、应急、通知与审计入口。

#### Scenario: Administrator without a write grant
- **WHEN** 管理员查看他人空间
- **THEN** 可读取和下载但不能直接修改，并可进入授权中心申请

### Requirement: Owner-controlled account recovery
Web SHALL 只向本人显示领取证明、新密钥与恢复码。

#### Scenario: Manual verification
- **WHEN** 最高管理员核验恢复申请
- **THEN** 页面仅显示恢复对象与请求 ID，不显示新凭据
