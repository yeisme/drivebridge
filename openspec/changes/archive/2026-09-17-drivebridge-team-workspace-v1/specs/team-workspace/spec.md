## ADDED Requirements

### Requirement: 团队变更共享权限与版本规则

服务 SHALL 在新旧入口保护最后一位团队管理员和最后一位空间数据负责人，新 team 动作必须校验 expected_revision。

#### Scenario: 陈旧修改

- **WHEN** 操作者提交旧版本号
- **THEN** 返回 409，团队和权限状态不改变

#### Scenario: 旧入口移除最后负责人

- **WHEN** 旧 team-member 或 permission 请求会失去最后负责人
- **THEN** 操作被拒绝且提示交接

### Requirement: 团队存储显式分配

服务 SHALL 仅允许平台或当前管理员在当前租户的自有 S3 兼容连接分配给团队，访问时校验具体空间分配。

#### Scenario: 连接仍被网盘引用

- **WHEN** 管理员请求取消仍被使用的分配
- **THEN** 返回冲突并保留绑定和文件

### Requirement: 归档只读且可撤销

服务 SHALL 保留原成员读取权限，拒绝归档团队的新写入，并撤销分享、临时授权和 Agent 访问。

#### Scenario: 归档期间完成上传

- **WHEN** 上传在团队归档后尝试提交
- **THEN** 不发布新文件；原操作者可取消并释放预留容量

#### Scenario: 恢复团队

- **WHEN** 租户或平台管理员恢复团队
- **THEN** 原长期空间权限恢复有效，旧分享、临时授权与 Agent 不自动复活

### Requirement: 用户绑定邀请

服务 SHALL 将邀请绑定指定用户与团队，仅在租户成员关系有效且邀请未过期时接受。

#### Scenario: 他人尝试接受

- **WHEN** 非指定用户提交邀请 ID
- **THEN** 服务拒绝且不创建成员关系
