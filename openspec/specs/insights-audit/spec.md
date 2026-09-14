# insights-audit Specification

## Purpose
TBD - created by archiving change drivebridge-local-storage-browser-v1. Update Purpose after archive.
## Requirements
### Requirement: 本机只读存储浏览

本机只读视图 SHALL 支持按空间浏览文件，实时列表由持有 state 锁的 owner 经 insights socket 委派执行，owner 缺席或拒绝时 SHALL 降级为已观察索引并如实说明原因。

#### Scenario: owner 运行时的实时目录
- **WHEN** owner 以 `--enabled --insights` 运行且 view 请求某空间目录
- **THEN** 返回实时列表（`coverage=live_listing`），观察结果进入索引；超过 500 条时截断并标记 `partial`

#### Scenario: owner 缺席的诚实降级
- **WHEN** insights socket 不存在、owner 未启用文件操作或列表失败
- **THEN** 视图不伪造实时结果，按已观察索引呈现并显示对应原因；不撞 state 锁、不新增写入口

#### Scenario: 路径与越界
- **WHEN** 请求携带 `..`、绝对路径、未 Clean 路径或未知空间
- **THEN** 返回 `path_not_allowed`/`space_not_allowed`，不触碰远端

#### Scenario: 目录发现不越出索引
- **WHEN** 远端存在从未被观察过的子目录
- **THEN** 该目录不出现在聚合导航中；目录聚合只来自已观察索引的 Dir 字段

### Requirement: 如实的存储后端标识

本机只读视图的空间投影 SHALL 为每个空间附带归一的后端身份，且不在视图中升级任何后端能力。

#### Scenario: 混合后端空间列表
- **WHEN** 本机 state 同时存在 OneDrive 空间（Backend 为空或 `onedrive`）与 generic rclone 空间（如 `s3`、`webdav`）
- **THEN** `/api/v1/spaces` 分别返回 `backend: onedrive` 与对应 generic kind，其余字段保持不变

#### Scenario: 视图不新增能力
- **WHEN** 任何后端的空间出现在本机视图
- **THEN** 文件操作保持只读，直链与参考交接能力标识只按既有 `backend_capabilities` 矩阵解释，generic 空间不得显示为支持直链或交接

### Requirement: 可解释且隔离的统计
系统 SHALL 区分有效与观测字节、托管与目录容量，并按当前身份权限查询，未知历史保持缺失。

#### Scenario: 缺失或受限数据
- **WHEN** 历史未采集、配额未知或 Agent 仅允许部分目录
- **THEN** 系统不伪造零值、不暴露空间总量、不将 caller 当作认证身份

### Requirement: 有预算的明确扫描
系统 SHALL 默认关闭扫描，使用有限条目和时间预算，保留最近完整结果。

#### Scenario: 扫描中断
- **WHEN** 扫描超限、取消或失败
- **THEN** 返回不完整状态，最近完整结果不被替代，只有支持检查点的运行可续扫

### Requirement: 可验证的安全审计
系统 SHALL 按当前权限暴露脱敏审计，关键操作依赖审计意图持久化，紧急撤销单独处理。

#### Scenario: 审计不可用
- **WHEN** 审计写入失败
- **THEN** 关键授权、覆盖和删除被拒绝，普通读取可降级，撤销仍尝试修改权限并如实报告数据库失败

### Requirement: 可访问的展示与兼容性
系统 SHALL 复用既有文件操作和输出合同，提供可筛选的图表、数据表与明确覆盖范围。

#### Scenario: 本机或远程查看
- **WHEN** 用户打开统计入口
- **THEN** 数据源保持分离，图表可用键盘及移动端查看，查询不会自动开启扫描或产生递归统计事件

