## ADDED Requirements

### Requirement: 统一存储可视化

网页与本机只读视图 SHALL 按后端身份呈现空间与连接，能力标签不得超出既有 `backend_capabilities` 矩阵，未识别的存储类型 SHALL 诚实降级而不是冒充已知类型。

#### Scenario: 混合后端空间徽标
- **WHEN** 本机视图的空间投影携带 `backend`（如 `onedrive` 与 `s3`）
- **THEN** 每个空间显示后端标签与对应能力（OneDrive=传输·临时直链·参考交接；generic=仅传输），服务端 `/spaces` 未携带 `backend` 时不猜测、保持静默

#### Scenario: 未识别存储类型
- **WHEN** 连接或空间出现客户端未收录的 kind
- **THEN** 显示「未识别的存储类型/后端」并按既有授权呈现，不静默归为 S3 或 OneDrive，不阻断浏览

### Requirement: 本机只读存储浏览

本机只读视图的存储浏览 SHALL 区分实时目录与已观察索引两种数据源并如实标注，浏览不得提供任何写入口。

#### Scenario: owner 运行时的实时目录
- **WHEN** owner 以 `--enabled --insights` 运行且用户浏览某空间目录
- **THEN** 显示实时列表并标注「列表会更新已观察索引」；超限截断时提示 partial

#### Scenario: owner 缺席的诚实降级
- **WHEN** owner 未运行、未启用文件操作或列表失败
- **THEN** 降级为已观察索引并显示对应原因与「非全盘」声明，不伪造实时结果

#### Scenario: 未观察目录不出现
- **WHEN** 远端存在从未被观察过的子目录
- **THEN** 目录导航不包含该目录；用户可通过手动路径输入访问已知路径，输入 `..` 等逃逸段被拒绝
