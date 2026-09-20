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

### Requirement: 本机文件复制
系统 SHALL 通过 `files/action` 的 `copy` 复制固定引用到目标目录。SHALL NOT 覆盖已有名称。服务端 Copy 不可用时 SHALL 回退为读取再上传。

#### Scenario: 同目录创建副本
- **WHEN** 用户复制一个本机文件到同一目录
- **THEN** 出现自动改名的新引用，源文件不变

### Requirement: 本机目录前缀操作
系统 SHALL 支持 `copy_directory`、`move_directory`、`trash_directory`。根路径或移入自身 SHALL 返回 `invalid_directory`。超过 1000 个文件 SHALL 返回 `directory_too_large`。

#### Scenario: 复制文件夹
- **WHEN** 源目录含文件
- **THEN** 目标出现新文件夹和其中文件，源目录保持不变

