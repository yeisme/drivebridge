## Why

用户在浏览器里查看本机存储只能看统计，不能按空间浏览文件内容清单；「人用 client 可视化」缺最后一块——只读存储浏览。owner 运行时希望看到实时目录，owner 缺席时降级为已观察索引并如实说明原因。

## What Changes

- insights socket 新增 `POST /list`（`ListRequest{space_id, path}`）：由持有 state 锁的 owner 调 `bridge.List` 实时列目录并入索引（与扫描同款观察语义），60s 超时、500 条截断 + `partial`、`coverage=live_listing`；错误码 `invalid_list/space_not_allowed/path_not_allowed/listing_disabled/list_failed`。
- view 新增代理路由 `POST /api/v1/local/list`（既有 Origin+CSRF 门覆盖）；socket 缺席直接 `insights_owner_unavailable`（503），不 fallback 撞锁。
- `QueryInsights` files kind 增量：`dir` 精确过滤（参数存在且空=根目录）与 `dir/mime_type/observed_at/trash_path` 投影字段（Tier 1 索引浏览的数据面）。
- **目录导航约束（实现发现）**：两个 provider 的 `List` 都只返回文件对象（目录被过滤，`remember` 要求 Size>0）——子目录发现只能来自索引 Dir 聚合，未观察过的目录不出现；client 以面包屑+手动路径补充（见 client change）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `insights-audit`：ADDED「本机只读存储浏览」需求（两级数据源、诚实降级、列表不新增写能力）。

## Impact

owner 进程新增一个只读委派端点（列表写索引=观察，不写远端）；view 无新增写入口；`/usage/files` 投影 additive。远程网站不受影响。
