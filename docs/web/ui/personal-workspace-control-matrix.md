# Personal Workspace 控件验收矩阵

本矩阵冻结于 2026-10-09 的 OpenDesign 只读快照。快照只用于视觉与交互输入；正式行为仍以 DriveBridge Server、Web 现有合同和本地 fixture 验证为准。矩阵不授权写入 OpenDesign、真实云空间或生产环境。

## 依赖与证据

| 项目 | 当前约束 | 证据 |
| --- | --- | --- |
| 原型入口 | `drivebridge-personal-agent-workspace`，执行时读取并固定摘要 | `temp/integration-test-runs/20261009T155631Z-f93c27ed/artifacts/prototype/000-index.html` |
| 直接资源 | 入口及 25 项 script/stylesheet 资源，逐项保存；不递归字体和动态导入 | `temp/integration-test-runs/20261009T155631Z-f93c27ed/artifacts/prototype/snapshot-manifest.json` |
| 生产边界 | 不复制编译 JS/CSS，不把原型状态当业务数据 | `docs/web/ui/notebook-workspace-production-spec.md`、`docs/web/ui/personal-workspace-reference.md` |
| 行为合同 | 固定版本、原操作恢复、权限、失败与 unknown 状态由 owner API 决定 | `docs/web/ui/personal-workspace-ui-spec.md`、`openspec/changes/drivebridge-personal-workspace-rebuild-v1/design.md` |

## 页面与控件矩阵

| 页面/区域 | 必验控件 | 必验状态与边界 | 最小证据 |
| --- | --- | --- | --- |
| 工作台首页 | 空态、最近项、集合、待处理、用量入口 | loading / empty / partial / stale / offline；固定引用跳转不丢 scope | Vitest + fixture 浏览器 |
| 文件导航 | 连接/root、面包屑、筛选、排序、四视图、目录抽屉 | 换 scope 取消旧响应；缺权、空目录和分页可恢复 | Files/Finder 定向测试 + 浏览器 |
| 文件列表 | 行、选择框、分页、原位工具条、预览入口 | 单击/双击/Enter；当前页多选；不把全量选择扩成万项本地状态 | Finder 组件测试 + fixture |
| 上传与恢复 | 拖拽/文件夹/手机 picker、摘要、发送、耐久、验证、发布 | 空文件、配额、取消、断连、丢创建响应；未知结果只查原键 | transfer 测试 + 原键恢复链 |
| 固定版本阅读 | 上下项、页码/缩放、专注、下载、复制引用、详情 | 历史版本失效不换最新版；安全解析、按需加载；切详情不重载正文 | reader 测试 + 多格式 fixture |
| 详情与版本 | 详情/版本/活动/关联页签、固定引用、讨论/私有备注 | 局部失败、晚响应、权限过滤；详情切换保持正文 ref | InfoWorkspace/Detail 测试 |
| Agent 工作台 | 接入表单、期限、目录/动作、卡片、分组、置顶 | 默认只读；never/duration 明示；撤权/失权清除；不展示演示调用 | Agent 组件 + grant fixture |
| 任务与收件 | 原操作状态、固定引用、预览、人工决定、归档 | pending/accepted/rejected/unknown；原回执恢复；ID 不取代文件名与状态 | inbox/task 组件 + HTTP/DB fixture |
| 知识与资源 | 来源筛选、笔记列表、Skill/模板/资料包详情、检查入口 | 固定 source/object/revision；检查不等于保存、安装或执行；Pinax 写入口禁用 | notebook/resource 组件 + 合同 fixture |
| 设置/存储/回收 | 连接表单、能力说明、设备、回收站、外观偏好 | 配置、权限、连接观察分开；危险清理仍禁止永久删除；失败可恢复 | 管理页组件 + 六视口浏览器 |
| 全局壳层 | 侧栏、三组导航、顶栏搜索、详情/对话框、手机五栏 | 320–1920px、浅深色、键盘返焦、reduced-motion、44px 触控；旧 URL alias 保留 | typecheck/build + 六视口矩阵 |

## 统一验收规则

1. 生产代码只能消费正式合同；原型资源不得出现在生产 import 或 bundle。
2. 所有写操作必须保留原 scope、固定版本、幂等键和 operation/receipt；未知结果先回查，不换键重试。
3. `empty`、`unavailable`、`forbidden`、`unknown`、`stale` 和 `offline` 必须分开呈现，不能用空列表掩盖失败。
4. 浏览器证据必须等待稳定文本或有效控件；仅挂载、networkidle、无溢出或静态截图不能关闭业务门。
5. 本矩阵只冻结设计输入和验收口径；真实云、Pinax 私有 vault、三平台实机、签名/发布和付费 Provider 仍由各自门负责。

## Capability release labels

| Capability | Stable | Experimental | Fixture-only | Planned |
| --- | --- | --- | --- | --- |
| OneDrive local path | target |  | local auth fixture |  |
| Generic rclone |  | transfer-only | provider fixture |  |
| Scaena handoff |  | local relay | relay fixture | production handoff |
| Remote multi-tenant |  | guarded | demo fixture | production tenancy |
| Electron reader |  |  | host fixture | three-platform release |

The UI must not present experimental or planned capabilities as production
features. Unknown provider or deployment states remain explicit and recoverable.
