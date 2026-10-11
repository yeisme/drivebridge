# insights-audit Specification

## Purpose
TBD - created by archiving change drivebridge-insights-audit-v1. Update Purpose after archive.
## Requirements
### Requirement: 可解释且隔离的统计
系统 SHALL 区分有效与观测字节、托管与目录容量，并按当前身份权限查询，未知历史保持缺失。

#### Scenario: 缺失或受限数据
- **WHEN** 历史未采集、配额未知或 Agent 仅允许部分目录
- **THEN** 系统不伪造零值、不暴露空间总量、不将 caller 当作认证身份

### Requirement: 有预算的明确扫描
系统 SHALL 默认关闭扫描，使用有限条目和时间预算，保留最近完整结果。

#### Scenario: 扫描中断
- **WHEN** 扫描超限、取消或失败
- **THEN** 返回不完整状态，最近完整结果不被替代，只有支持检查点的运行可续扫

### Requirement: 可验证的安全审计
系统 SHALL 按当前权限暴露脱敏审计，关键操作依赖审计意图持久化，紧急撤销单独处理。

#### Scenario: 审计不可用
- **WHEN** 审计写入失败
- **THEN** 关键授权、覆盖和删除被拒绝，普通读取可降级，撤销仍尝试修改权限并如实报告数据库失败

### Requirement: 可访问的展示与兼容性
系统 SHALL 复用既有文件操作和输出合同，提供可筛选的图表、数据表与明确覆盖范围。

#### Scenario: 本机或远程查看
- **WHEN** 用户打开统计入口
- **THEN** 数据源保持分离，图表可用键盘及移动端查看，查询不会自动开启扫描或产生递归统计事件

