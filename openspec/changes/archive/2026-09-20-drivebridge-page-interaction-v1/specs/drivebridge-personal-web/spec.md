## ADDED Requirements

### Requirement: 楔子页面共用列表手感
传输、收件箱、Agent、分享列表 SHALL 可键盘移动焦点。空状态 SHALL 提供一个下一步按钮。取消传输 SHALL 先确认。

#### Scenario: 传输空状态
- **WHEN** 没有传输记录
- **THEN** 显示「去网盘上传」

#### Scenario: Agent 筛选
- **WHEN** 用户选择「有效」
- **THEN** 只显示未撤销且未到期的 Agent
