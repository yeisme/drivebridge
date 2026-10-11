## MODIFIED Requirements

### Requirement: Tenant-aware workspace

Web SHALL 按租户、团队和空间权限展示页面，管理访问默认只读，所有写入由服务端授权。账号、租户或管理范围切换时 SHALL 清空旧详情、选中项、表单和敏感一次性结果，并拒绝旧上下文响应。

#### Scenario: Tenant switch during an in-flight request

- **WHEN** 用户切换租户或空间
- **THEN** 清空旧视图并拒绝旧上下文响应，取消在途传输，旧抽屉与输入不出现在新上下文

### Requirement: Approval and notification center

Web SHALL 在独立管理页面提供申请与审批、通知、访问记录、租户与权限、账号与恢复五类入口，并根据身份展示可用名称和动作。申请、详情、应急和恢复 SHALL 复用现有权限合同，审批 SHALL NOT 自动执行清单。

#### Scenario: Administrator without a write grant

- **WHEN** 管理员查看他人空间
- **THEN** 可读取和下载但不能直接修改，并可进入独立授权中心申请

#### Scenario: Approve and explicitly execute

- **WHEN** 审批人批准具体操作清单
- **THEN** 列表更新为已授权，只有原申请人明确确认后才请求执行，部分失败保留原清单的逐项结果，说明成功项保留且失败项核对最新版本后重新申请，不重放一次性清单

#### Scenario: Open a task form on demand

- **WHEN** 用户选择创建、邀请或修改
- **THEN** 打开桌面 480px、手机适配全宽的表单抽屉，列表页不常驻展开全部表单；失败保留输入并在抽屉内解释

#### Scenario: Reuse team management

- **WHEN** 用户从管理页选择管理团队成员与网盘
- **THEN** 到达同租户的现有团队工作区；租户邀请、授权、恢复、应急等既有能力仍有明确入口

#### Scenario: Close a sheet with a keyboard

- **WHEN** 用户用 Escape 或取消关闭未提交的抽屉
- **THEN** 不发送写请求，焦点回到原触发器，列表可以继续键盘操作
