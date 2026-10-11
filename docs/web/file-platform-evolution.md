# 文件工作台、数据可视化与操作流程（设计）

2026-09-30 后续设计：本文件保留既有信息工作台/文件流程的合同与实施历史。新版主导航、页面壳层和四页签详情改由 [个人多 Agent 工作台 PRD](product/personal-agent-workspace-prd.md) 与 [UI 规格](ui/personal-workspace-ui-spec.md) 定义；旧六页签内容、上传恢复和固定交付语义须完整迁入，旧路由继续兼容。新任务归 [前端重建包](../../openspec/changes/drivebridge-personal-workspace-rebuild-v1/README.md)，不以历史任务状态代替新版验收。

## 入口和状态

本文件是新增体验的设计导航，不是运行验收。现有Finder、预览、容量与审计成果保留；总览和流程增量分别由两个本地change持有：

- [信息工作台](../../openspec/changes/drivebridge-information-workspace-v2/design.md)
- [上传/检索/整理/交付流程](../../openspec/changes/drivebridge-file-platform-consumers-v2/design.md)

## 导航与信息层次

沿用工作台/协作/系统三组。工作台提供总览、文件、搜索、传输、收件箱和整理；协作继续分享、团队与回收站；连接、机器人授权和策略管理保持系统组。领域生成/审核不搬进DriveBridge。

总览六组信息：容量、增长、传输、连接、整理、Agent活动。默认显示用户当前可读范围；每张图标明口径、观测时间、覆盖和延迟，能回到同筛选明细，数据表是同等入口。未知值显示“未取得”，部分扫描显示范围，不以0代替。

文件详情按概览、版本、来源与引用、内容理解、讨论、活动组织。固定ref不因页面刷新变最新版；来源观测/受管副本/缓存/远端未刷写分别说明。当前动作由服务端capability决定，不以UI按钮存在推断允许。

## 主要交互

| 页面 | 正常流程 | 失败与恢复 |
| --- | --- | --- |
| 上传中心 | 目录预检→摘要→批次→耐久进度→验证→发布 | 原键lookup、重选文件、逐项partial、暂停/取消分开 |
| 云端浏览 | 选择已授权连接/root→分页→当前观测→明确导入 | 断连、source_changed、export_required、cursor_expired |
| 搜索探索 | 元数据exact、内容Top-K、片段证据分组 | 缺模态/预算/索引过期明确，不自动分析 |
| 整理中心 | 查询固定集合→计划before/after→授权→逐项结果 | plan_stale重新预览；部分成功不重做；补偿冲突保留 |
| 交付页 | 固定成员/清单→全部校验→发布→接收事实 | 缺项不可完整；撤销不声称召回副本 |
| 讨论 | 旧版本锚点→纯文本线程→解决/重开 | revision冲突保留草稿；私人备注公开先预览 |

中文状态必须区分“上传中”“验证中”“已保存”“已传出”“已确认接收”“领域已采纳”。客户端只能展示对应owner的事实，不自行推断转换。

## 响应式与无障碍

复用DataTable、DirectoryLayout与现有图表原语；320/390px单列/详情抽屉，768px以上逐步双栏。44px触控目标、明确焦点、键盘可操作和减少动效；图表可切数据表，状态不只用颜色。

URL只存安全筛选与选择，不放凭据/内容。空间切换取消旧请求并按context revision丢弃晚到结果。分页/虚拟列表避免万项同时渲染，后台页面暂停轮询。IndexedDB仅存必要非秘密上传恢复身份，不持久文件正文或长期token。

本机网页只有私有owner确认可写才展示对应既有动作；owner缺席为只读索引。与服务端体验共用组件，不复制存储/权限规则。

## 验收

每个页面覆盖loading/empty/partial/stale/forbidden/offline/unsupported/error与恢复；至少320px/390px/768px/1440px、键盘、焦点恢复、两用户撤权及空间切换晚响应。百万目录和万批次使用分页服务fixture，不能一次性载入全部数据。

```bash
pnpm test
pnpm build
pnpm test:e2e
```

证据由现有runner生成到本owner temp/integration-test-runs/<run-id>/六件套。内容质量、云端吞吐和模型成本分别归Server/Manju/领域owner验收；浏览器fixture不宣称真实服务已启用。

## 实现状态与回滚（信息工作台，2026-09-25）

本节只记录已发生的事实，不把计划写成已上线：

- 已落地（本地验证）：`#/overview` 信息工作台页面与工作台导航入口；总览六组面板（容量/增长/传输/连接/整理/Agent活动）按口径、观测时间与覆盖标注；同口径钻取（容量明细/传输事实/精确文件清单）；文件详情六页签与固定版本选择；原操作按原ID回读；上下文切换丢弃晚到响应；URL只承载安全筛选。单元与浏览器验证命令为上面的 `pnpm test`、`pnpm build`、`pnpm test:e2e`。
- 联调漂移修正（2026-09-26，首轮真实后端冒烟后）：整理面板与能力探针显式发送 `consistency:"live"` 并固定携带 `dimensions:["mime_type"]`/`metrics:["logical_bytes","file_count"]`（后端默认 observed 离线返回 503 `analytics_backend_unavailable`，双空 selection 返回 400 `invalid_query`；observed 503 在能力门按未部署诚实降级）；analytics `rows:null` 归一化为空列表；content-bindings 请求改按 `space_id` 列表并在客户端按 `file_ref` 收窄，归一化读实装 `binding_id/file_ref/state`（真实绑定不再被丢弃）；threads 归一化接受 `version_id:""` 文件级锚点（design.md:43 默认 file 级），详情与交付中心显示“文件级”。e2e mock 同步为实装响应形状。
- 回滚：信息工作台是纯增量页面；移除方式是从导航组与路由表中删除 `overview` 入口并删除 `src/info-workspace/`，不影响既有页面与接口。`src/api.ts` 的信封解析容错（非JSON错误体按 `request_failed` 处理）为共享修复，回滚它只会恢复旧的解析报错，不产生数据影响。无数据迁移、无凭据存储变更。

## 实现状态与回滚（文件流程，2026-09-25）

本节只记录已发生的事实，不把计划写成已上线：

- 已落地（本地验证）：`#/flows` 文件流程页与工作台导航入口；四个中心（上传中心/云端检索/整理中心/交付与讨论）按冻结合同接入 typed client；上传批次创建前持久化非秘密幂等身份（IndexedDB，按 instance/actor/space 隔离，登出/切换身份清理）；原键 lookup 恢复与重选文件 SHA-256 校验（不一致保留已验证进度）；分阶段进度（摘要/已发送/已耐久/已验证/发布，100% 已发送显示“验证中”而非完成）；检索区分精确列表与 Top-K 候选并展示覆盖/缺失模态/索引代次；整理计划物化快照→预览→expected_revision 确认，plan_stale 强制重新预览；固定交付缺项禁发并给出恢复入口，接收事实五级分级不升格；版本讨论锚定固定版本、草稿失败保留、私人备注公开前预览。本地验证命令为 `pnpm test`、`pnpm build`、`pnpm test:e2e`。
- 联调漂移修正（2026-09-26，首轮真实后端冒烟后）：organization-policy 视图读实装 `revision` 字段，缺策略 404 `not_found` 归一化为未配置常态并引导创建（不再弹错误横幅）；private-note publish-preview 读实装 `body_preview/note_revision/note/target`（不再读不存在的 `target_teams/body`）。e2e mock 同步为实装响应形状。其余 5 项漂移（D3/D5/D7/D10/D12）归 backend 侧修复，联调复验由统一冒烟执行。
- 回滚：文件流程是纯增量页面；移除方式是从导航组与路由表中删除 `flows` 入口并删除 `src/consumer-flows/`，不影响既有页面与接口。恢复身份存于浏览器 IndexedDB（库 `drivebridge-consumer-flows`），删除该库即清理；讨论草稿键 `drivebridge:consumer-flows:draft` 同理。无服务端数据、无凭据存储变更。
