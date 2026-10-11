## RENAMED Requirements

- FROM: `### Requirement: 分享与创作入口`
- TO: `### Requirement: 分享与固定引用`

## MODIFIED Requirements

### Requirement: 分享与固定引用
系统 SHALL 提供固定交付、持续共享、授权管理与固定版本引用复制。个人网盘、团队网盘及本机网页 SHALL NOT 提供创作入口、提示词编辑、@ 创作素材选择、模型连接配置或生成提交面板。

#### Scenario: 网盘文件操作
- **WHEN** 用户进入个人、团队或本机网盘，或选择文件
- **THEN** 页面提供文件操作与预览，不出现“用于创作”或生成面板

#### Scenario: 只插入引用
- **WHEN** 用户复制文件引用并插入外部应用
- **THEN** 引用保留所选版本，不自动授予访问权限或提交生成；网盘内不提供 @ 创作素材面板

#### Scenario: 分享撤销
- **WHEN** 用户撤销分享
- **THEN** 接收方后续访问失败且源文件不被删除
