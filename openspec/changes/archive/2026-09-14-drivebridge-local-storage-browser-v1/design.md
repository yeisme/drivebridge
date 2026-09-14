# Design

## 两级数据源

| Tier | 条件 | 数据 | 诚实标注 |
|---|---|---|---|
| 2 实时 | owner `mcp --enabled --insights` 运行，socket 在 | `bridge.List` 实时目录 | `coverage=live_listing`；列表会更新已观察索引 |
| 1 索引 | socket 缺席 / owner 未启用 / 列表失败 | `QueryInsights files`（`dir` 过滤） | `coverage=observed_index_verify_with_stat`；索引≠全盘 |

## 关键决策

1. **委派而非直连**：`bridge.List` 要求 `s.Enabled` 且需持 state 锁；view 是只读无锁进程。实时列表必须经 `insights.sock` 由 owner 执行（与 `/scan` 同款模式）。socket 缺席时 view 直接报 `insights_owner_unavailable`，不 `openService` fallback（会撞锁且无 Enabled 语义）。
2. **路径安全透传**：`remoteDir` 已拒绝 `..`/绝对/未 Clean 路径，socket handler 只做 `space_id` 非空与 `path≤1024` 校验，错误码映射 `path_not_allowed`。
3. **截断与超时**：双端 60s；服务端 500 条截断 + `partial:true`（大目录不撑爆 view 的 4MB 响应限读）。
4. **错误码矩阵**（socket→view→client 降级文案）：

| 场景 | socket code | view | client |
|---|---|---|---|
| body 坏/参数缺 | invalid_list | 400 | 提示参数错误 |
| 空间无效 | space_not_allowed | 400 | 提示空间无效 |
| 路径非法 | path_not_allowed | 400 | 提示路径非法 |
| owner 未 --enabled | listing_disabled | 400 | 降 Tier1「实时浏览需 owner 启用文件操作」 |
| provider 失败 | list_failed | 400 | 降 Tier1 |
| socket 缺席 | — | 503 insights_owner_unavailable | 降 Tier1「未检测到运行中的 owner」 |

5. **目录发现约束**：`Generic.List`/`OneDrive.List` 均只返回 `fs.Object`/Graph `file` 条目（目录被过滤；`remember` 因 Size≤0 拒绝目录行）。子目录列表由 client 从索引 `Dir` 字段聚合得出，未观察目录不显示——与「索引≠全盘」一致，不扩 Provider 合同。

## 不做

- 不做 `/stat` 委派（Tier 2 的 remember 已刷新该行；stat 另需评估）。
- 不做目录行的 Provider 扩展（破坏 File/remember 不变量，收益低）。
- 远程（remote_url）模式不加 list：远程网站已有文件页。
