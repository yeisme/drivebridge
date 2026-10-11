# 实施设计与对接

## 目标与约束

用户已确定紧凑文件客户端、独立管理页、全站共享规范。第二轮反馈进一步明确：紧凑不能压缩文字内边距，必须补充列表详情模式及分栏、画廊阅读。交互尺寸、响应式和验收矩阵的详细定义统一放在 [UI 规范](../../../../docs/web/ui/compact-workspace.md)，本文件维护实现边界与依赖。

## 能力归属

| 能力 | 判定 | 唯一 owner / 对接 |
| --- | --- | --- |
| 控件、表格、表单、目录与详情布局 | fit | 本 client 的共享组件与工作区页面 |
| 管理页、账号详情、权限矩阵 | fit | 本 client 呈现现有 Server actions、revision 和错误 |
| 分栏/画廊选择及完整阅读 | fit | Finder 复用现有 Media/Quicklook 和固定版本上下文 |
| 容量数值与历史采集 | split-owner | Server 保持数值口径；client 仅改排版与显示 |
| 账号开通、权限授予、存储挂载、格式转换服务 | reject-now | 复用已交付合同，本 change 不新增或改变这些服务能力 |

## 组件结构和数据流

```text
App shell / tab-local tenant + managed-space context
  ├─ Governance page → category route → table/list → detail or form Sheet
  ├─ Drive / team directory → filtered list + selected detail → existing full route
  ├─ Finder → navigation + view/focus → search → commands → shared file selection
  │                                                      └─ existing full reader
  └─ Insights → summary strip / container-aware chart + snapshot list

Server projection → scoped API client → UI state → explicit user action → existing API
```

共享 Button/Input/Select 保持原参数可调用，默认控件 32px；复合内容使用独立 surface 布局。DataTable 复用原引擎，新增可选列对齐、列宽及外部分页模式；旧调用默认内部分页。业务表格的留白和语义边框在共享层定义，页面不得用裸 `border-b` 依赖 currentColor，也不得用全局 `button` 高度修补复合内容。

AppDialog、Sheet 共享表单节奏和操作区规则。Sheet 使用已有 Base UI 焦点管理；字段值只在本次编辑中保留，失败不关闭，提交期间禁用重复操作。含凭据的一次展示面继续由原流程持有，不进入列表、URL、截图或本地持久化。

### 路由与状态

- 新增 `#/governance?tab=requests|notifications|events|teams|accounts`，携带可选 `tenant` 与 `space`；这里 `space` 明确表示管理访问范围。未知 tab 回到 requests，未知或无权访问租户显示拒绝，不悄悄退回更大范围。
- 系统导航、工作区菜单和用户目录的租户设置进入同一管理页。原 `#/users`、`#/members`、`#/analytics` 及 `#share/...` 行为保持。
- 目录入口 `#/drives`、`#/teams` 接受可选 `layout=list|cards` 与 `selected=<id>`。未指定布局默认列表详情；卡片模式保留原单击进入行为。选择行更新当前 URL 的 selected，不为每次选择新增历史；进入完整工作区新增历史，返回时恢复原查询、选中项和仍有效的滚动位置。
- selected 仅匹配当前可访问且通过筛选的结果；被过滤、删除、撤权或切换租户后清空，不能用参数直接加载未获授权对象。首屏默认选中首个可见项目，不额外持久化 ID。
- 专注沿用 `view=drive`，只增删此参数，保留 tenant/team/managed scope 等上下文；不借新窗口提升权限。
- 所有在途查询使用原 API 上下文版本检查及取消机制。切换账号、租户、管理范围后，表单、详情、选择、错误和敏感一次性结果随旧组件卸载；旧响应不得写回新界面。

### 目录与阅读对接

网盘列表使用已有 spaces、connections、governance 投影；团队列表使用已有团队列表和详情 API；用户目录复用已有用户详情。摘要仅展示当前权限可见字段，未知值不能转成零。进入详情不额外列举其他租户资源。

Finder 四视图复用同一排序、筛选和选择来源。分栏行和画廊项不得套普通按钮的固定高度或居中单行样式。单击选择、双击/Enter 打开完整阅读器；文件夹选择和进入仍遵循既有合同。当前阅读器支持的格式直接复用；转换待就绪、加密或不支持格式显示真实状态与下载入口，不为解决 UI 问题新增转换服务。

### 一次性审批清单恢复

已核对 Server 的 execute 合同：只接受仍为 approved 的原申请人请求，执行前原子领取一次，partial/interrupted 不能重放。前端保留原清单及逐项成功/失败回执，提示用户核对失败项与最新版本后提交新的审批清单；不得显示服务端不支持的“继续执行”按钮。普通文件传输仍沿原 operation 恢复，此处不混淆两个合同。写入成功但随后刷新失败时，提示结果已提交并重读列表，不把已成功写入当成可重复提交的失败。

### 分页归属

通用本地表格只保留一套内部分页；消费服务端分页的用户/团队/权限列表使用可选外部分页模式，关闭本地第二套翻页与页大小选择。权限矩阵的成员行与网盘列是两个独立维度，分别明确命名；单页且无下一页时隐藏该维度的无效翻页，不隐藏数据。不声称当前页搜索等于全量服务端搜索。

## 兼容、前置与回滚

| 表面 | 分类 | 兼容措施 |
| --- | --- | --- |
| HTTP、MCP、持久化、权限字段 | 不变 | 仅消费既有公开合同，无后端迁移 |
| hash 导航 | 增量 | 保留旧路径；新增参数不参与权限判定 |
| 共享 TS 组件 | 增量 | 新参数可选、原默认调用仍可工作；检查本机 view 与团队页面 |
| 文档/spec | 同 owner 增量 | 通过 change deltas 描述目标，验收前不提前改写主 specs 为已交付 |

`shadcn-full-ui` 是已有原语依赖。本 change 的语义令牌真实来源是 `src/shadcn.css`，旧 `style.css` 和 legacy 布局别名继续引用它。归档按前置 change → 本 change 顺序执行，确保规格不会回退。更新文档中“暗色主题非目标”与旧管理弹窗说明时标明历史范围。

回滚只回退本 change 的前端布局/路由提交并重建静态资源，保留用户数据、账号、密钥、主题/密度偏好和原有 API。旧前端忽略新增展示参数，原数据路径仍能打开；不修改本机私有状态或重建体验账号。

## 实施顺序

1. 先完成本轮文档、九个 capability delta 和 CLI 管理任务。
2. 修复共享表格语义边框、48px 最低行高、单元格内边距、表单 action row；用团队权限和用户目录作为首批样例。
3. 完成管理页与表单/详情抽屉，验证原审批、明确执行及租户隔离。
4. 完成网盘/团队列表详情、Finder 工具栏、专注、分栏及画廊阅读。
5. 完成容量与概览布局；补齐全部已定义状态与回归。
6. 稳定后运行完整门禁、截图比较与 OpenSpec 校验；成功证据通过任务 CLI 写入，之后才能归档。

## 验收与收口

两轮十条反馈已实现。普通、治理与团队 fixture 分别验证；默认 fixture 的条件跳过项已在专用实例中执行，接管和归档场景使用独立一次性实例。完整命令、逐项结果、失败修复归因及前后截图索引见 [本轮验收记录](../../../../docs/web/ui/compact-workspace-validation.md)。

七种宽度、深浅主题、侧栏状态、触屏点击区域、真实零值与缺口、失败输入、部分回执、固定版本阅读均有对应证据。服务端权限与一次性清单合同不变；未知或无权上下文不能将旧页面状态带入新页面。修改仅作用于本地 client，未发布或部署。

先归档已完成的 `drivebridge-shadcn-full-ui-v1` 原语前置，再归档本 change 的九个 capability delta。表格文件预览来自并行任务，其实现与文档不在此 change 的状态归档范围内。
