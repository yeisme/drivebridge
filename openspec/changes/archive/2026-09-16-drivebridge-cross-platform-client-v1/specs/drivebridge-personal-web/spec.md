## ADDED Requirements

### Requirement: 跨平台可安装

网页 SHALL 以同一份资产服务桌面与移动浏览器，并提供可安装的应用清单；不得为跨平台引入独立客户端壳。

#### Scenario: 手机安装为应用
- **WHEN** 用户在手机浏览器打开已部署的网页并选择「安装/添加到主屏」
- **THEN** manifest 提供名称、主题色与图标，应用以独立窗口打开；不新增运行时、更新通道或原生包

#### Scenario: 本机视图不参与安装
- **WHEN** 用户打开 `drivebridge view` 的本机只读视图
- **THEN** 该一次性启动页面不声明安装清单，保持 loopback 与一次性 fragment 语义

#### Scenario: 平台状态如实呈现
- **WHEN** 平台能力未经验证（如 Windows 实机）
- **THEN** 分发与文档按「未实机验收的候选」标注，不宣称可用
