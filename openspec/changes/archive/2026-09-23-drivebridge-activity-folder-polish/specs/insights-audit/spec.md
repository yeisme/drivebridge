## ADDED Requirements

### Requirement: 最近活动共享中文动作语义
客户端 SHALL 复用审计中文动作字典，并区分操作请求、完成和拒绝；未知动作与结果使用中性中文，保留原始值提示。

#### Scenario: 后台读取机器人授权
- **WHEN** 最近活动收到 agents.list
- **THEN** 展示查看机器人授权，不断言用户打开过页面
