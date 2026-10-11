# drivebridge-legacy-entry-retirement Specification

## Purpose
TBD - created by archiving change drivebridge-legacy-entry-cleanup-v1. Update Purpose after archive.
## Requirements
### Requirement: 唯一工作区入口
DriveBridge MUST 仅由 cli/drivebridge 作为工作区项目入口，MCP、Server 和 Web 共同管理。

#### Scenario: 开发者查看项目目录
- **WHEN** 旧入口清理完成
- **THEN** 原三个目录不再存在，本地 Git 配置和当前开发命令不再注册旧项目。

### Requirement: 清理保全
清理 MUST 保留合并后的源码、未提交修改、历史与功能任务状态。

#### Scenario: 旧路径不是预期软链接
- **WHEN** 清理脚本发现真实目录或目标不符
- **THEN** 在删除之前失败，禁止递归删除真实目录。

#### Scenario: 移除三个旧链接
- **WHEN** 预检查通过并执行清理
- **THEN** 统一仓库和 Server/Web 原 Git 备份保持可用，前端重建不被误标为完成。

