## ADDED Requirements

### Requirement: Structured workspace menu and settings outline
工作区菜单 SHALL 按身份、空间和导航分组，体验身份控件不挤压标签。设置页 SHALL 以大纲替代右侧使用偏好说明卡，保留实际外观和资料能力。

#### Scenario: Navigate settings sections
- **WHEN** 用户点击或键盘激活大纲项目
- **THEN** 对应分区进入视口并可获得焦点，现有 hash 路由和未保存字段不变

#### Scenario: Switch an experience persona
- **WHEN** 用户打开工作区菜单并选择体验身份
- **THEN** 标签、选择器与确认动作保持可读，提交沿既有身份切换和页面清理流程
