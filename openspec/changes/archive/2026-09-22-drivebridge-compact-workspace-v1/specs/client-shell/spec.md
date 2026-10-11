## ADDED Requirements

### Requirement: Standalone management navigation

客户端 SHALL 在“系统”组提供独立“管理与授权”页面，原工作区菜单和用户目录的管理入口 SHALL 到达该页面。页面 SHALL 支持 `#/governance` 的 tab、tenant 与管理空间上下文，刷新和历史导航保持当前分类。

#### Scenario: Open a management deep link

- **WHEN** 用户打开 `#/governance?tab=events&tenant=t&space=s`
- **THEN** 在原应用壳内展示有权访问的管理页面及访问记录分类，保持 t/s 的上下文，URL 不授予额外权限

#### Scenario: Unknown category and forbidden tenant

- **WHEN** tab 不被识别或用户无权访问指定租户
- **THEN** 未识别 tab 回到申请分类；无权访问租户明确拒绝，不退回到权限更大的工作区

#### Scenario: Legacy navigation still works

- **WHEN** 用户通过旧用户、Key、团队、网盘、分享或资源态势链接进入
- **THEN** 原链接仍到达原能力，新增管理页不替换它们的数据归属
