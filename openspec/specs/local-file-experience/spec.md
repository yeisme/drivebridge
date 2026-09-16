# local-file-experience Specification

## Purpose
TBD - created by archiving change drivebridge-local-file-experience-v1. Update Purpose after archive.
## Requirements
### Requirement: 私有文件操作
系统 SHALL 在原授权空间和私有 socket 内执行文件操作。

#### Scenario: owner 未启用
- **WHEN** 网页请求文件修改而 owner 未启用
- **THEN** 系统拒绝修改并保留索引浏览

### Requirement: 远程副本分享
系统 SHALL 先固定或同步远程副本，再通过远程服务发布分享。

#### Scenario: 固定版本源改变
- **WHEN** 固定交付的源引用内容发生变化
- **THEN** 系统拒绝以新内容替代旧引用

#### Scenario: 重复创建
- **WHEN** 原分享复制在中断后恢复
- **THEN** 系统复用上传与分享标识，不重复发布

