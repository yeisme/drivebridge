## Why

`drivebridge view` 的 `/api/v1/spaces` 只投影 `id/name/connection_id`，用户在可视化里看不出每个空间背后是什么存储（OneDrive、对象存储、NAS 协议……），也看不出不同后端的能力差异（直链/参考交接是 OneDrive 专属）。统一存储可视化需要后端身份作为第一等数据。

## What Changes

- view `/api/v1/spaces` 每个空间增补 `backend` 字段，取 `bridge.Space.Kind()`（`""`/`onedrive` 归一为 `onedrive`，其余为具体 generic 后端 kind）。additive，旧字段不变。
- 能力语义沿用既有 `backend_capabilities` 矩阵：`onedrive` = transfer + direct_url + reference_handoff；generic = 仅 transfer。view 不新增写能力。
- client 消费与渲染归 client owner 的统一存储可视化 change（见 `client/drivebridge-client`）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `insights-audit`：本机只读视图的空间投影补充后端身份（ADDED 需求，不改既有条款）。

## Impact

只读投影加字段；不触碰文件操作、扫描、审计与 owner socket。远程网站 `/spaces` 不变。
