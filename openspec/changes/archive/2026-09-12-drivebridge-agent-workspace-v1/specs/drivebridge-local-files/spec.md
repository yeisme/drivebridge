## ADDED Requirements

### Requirement: Workspace directories and transfer CLI
工作区 SHALL 支持目录组织与直连 CLI：目录参数限定在空间钉定根内，传输与组织复用既有操作/恢复/引用合同；CLI 是用户本人终端的等价入口。

#### Scenario: Organize and transfer by directory
- **WHEN** agent 或用户在空间内创建目录并指定目录上传、按目录列出
- **THEN** 目标名保持操作前缀（不替换既有文件），按路径列出的引用与根层引用同一稳定合同

#### Scenario: Path escape rejected
- **WHEN** 目录参数为绝对路径、含 .. 遍历或未清洗形式
- **THEN** 返回固定拒绝码，不触达空间外的远端路径

#### Scenario: mkdir idempotence
- **WHEN** 重复创建已存在的目录路径
- **THEN** 成功返回；同名文件占用该段时返回冲突而非覆盖

#### Scenario: CLI retry reuses the original operation
- **WHEN** 用户重复执行同一 CLI 传输命令而未显式提供幂等键
- **THEN** 确定性派生的键命中原 operation，恢复原传输而不创建重复文件
