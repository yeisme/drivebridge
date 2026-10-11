## ADDED Requirements

### Requirement: Stable filters and readable audit events
客户端 SHALL 使用不推动正常布局的高级筛选浮层，审计身份优先展示当前权限范围内已知名称，长标识首尾缩写，已知动作采用中文展示。原始标识、动作与导出合同 SHALL 保持。

#### Scenario: Open advanced filters
- **WHEN** 用户展开高级筛选或选择日期
- **THEN** 浮层保持在视口内，下方指标不因展开位移，筛选条件与快捷日期一致

#### Scenario: Unknown identity or action
- **WHEN** 当前投影没有名称或动作没有翻译
- **THEN** 显示类型与短标识或其他操作，不跨权限补查；原始值仍可读取
