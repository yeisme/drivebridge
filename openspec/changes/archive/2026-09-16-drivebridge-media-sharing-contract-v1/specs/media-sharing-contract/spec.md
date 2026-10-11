## ADDED Requirements

### Requirement: 安全媒体读取
系统 SHALL 校验文件权限与固定版本，并正确处理 Range。

#### Scenario: 非授权用户读取缩略图
- **WHEN** 另一用户请求私有预览
- **THEN** 系统拒绝读取且不返回源数据

### Requirement: 分享与删除保护
系统 SHALL 保留固定交付的版本集合并阻止删除仍被引用的版本。

#### Scenario: 文件已分享
- **WHEN** 用户请求永久删除仍有活动分享的文件
- **THEN** 系统返回引用影响并拒绝永久删除

### Requirement: 生成交接与回存
系统 SHALL 保留领域任务真相并独立恢复原产物的传输。

#### Scenario: 重复回存
- **WHEN** 相同原产物再次请求回存
- **THEN** 系统复用上传操作而不再次生成
