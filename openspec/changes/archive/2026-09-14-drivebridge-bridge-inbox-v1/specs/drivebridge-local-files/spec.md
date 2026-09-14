## ADDED Requirements

### Requirement: Recent operations inbox
generic 桥 SHALL 提供只读收件箱工具：列出本机状态库近 30 天内更新过的完成或失败操作，按更新时间倒序且有上限；不写任何状态、不触发传输或恢复。

#### Scenario: Bounded recent window
- **WHEN** 调用 inbox
- **THEN** 仅返回近 30 天更新的操作，按更新时间倒序，最多 200 条

#### Scenario: Honest failure
- **WHEN** 本机状态库不可读
- **THEN** 返回 state_unavailable，不以空列表冒充成功
