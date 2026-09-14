## ADDED Requirements

### Requirement: 如实的存储后端标识

本机只读视图的空间投影 SHALL 为每个空间附带归一的后端身份，且不在视图中升级任何后端能力。

#### Scenario: 混合后端空间列表
- **WHEN** 本机 state 同时存在 OneDrive 空间（Backend 为空或 `onedrive`）与 generic rclone 空间（如 `s3`、`webdav`）
- **THEN** `/api/v1/spaces` 分别返回 `backend: onedrive` 与对应 generic kind，其余字段保持不变

#### Scenario: 视图不新增能力
- **WHEN** 任何后端的空间出现在本机视图
- **THEN** 文件操作保持只读，直链与参考交接能力标识只按既有 `backend_capabilities` 矩阵解释，generic 空间不得显示为支持直链或交接
