## ADDED Requirements

### Requirement: Artifact inbox page
网页 SHALL 提供产物收件箱页：只读展示近 30 天操作结果（完成与失败），轮询刷新，失败项按错误码给出用户可理解的文案并可跳转传输中心恢复，不提供任何生成入口。

#### Scenario: Completed artifact
- **WHEN** 收件箱中的操作已完成
- **THEN** 展示产物名、来源与大小，可进入版本选择流查看固定版本

#### Scenario: Failed artifact
- **WHEN** 操作失败或中断
- **THEN** 按错误码显示中文文案，提供到传输中心的恢复入口，不在收件箱内重试传输

### Requirement: Managed archive form
版本面板 SHALL 支持把选中固定版本归档到目标空间与路径：提交走 `/archive`，路径冲突由服务端自动改名，用户不做覆盖确认；`invalid_archive` 等错误码有明确文案。

#### Scenario: Archive from pinned version
- **WHEN** 用户在版本面板选择目标空间与路径提交归档
- **THEN** 提交归档请求并以操作结果反馈，绝不覆盖既有文件的承诺由服务端保证
