# DriveBridge 单仓合并记录

用户在当前会话明确要求把多个 DriveBridge 子项目合并，解决开发管理分散问题。唯一维护 owner 变为 `cli/drivebridge`；本轮只做本地仓库、工具和文档迁移，没有创建远端、提交、推送或部署。

| 原路径 | 新路径 | 过渡 |
| --- | --- | --- |
| mcp/drivebridge | cli/drivebridge | 旧入口已移除，CLI/MCP 在根模块 |
| backend-server/drivebridge | cli/drivebridge/server | 旧入口已移除，同仓服务模块 |
| client/drivebridge-client | cli/drivebridge/web | 旧入口已移除，同仓网页模块 |

迁移时对 1847 个 tracked/非忽略 untracked 文件在移动后、调整链接前逐一比对 SHA-256，零不一致。原 ignored 的 node_modules、构建输出、原型与 temp 证据当时随目录整体搬移；后续历史项目残留清理见下文。

当前主仓 Git 历史仍保留。2026-10-01 用户明确要求彻底清理本地历史项目保留物，已删除 `.git/consolidation-backup`，包含原 Server/Web 独立 Git、旧 runtime 副本、父仓旧 index、旧 .gitmodules 与迁移清单。不再保留这些本地备份，也没有把原 Server/Web 历史导入当前 Git。

同时删除 `temp/consolidation-baseline`、`web/temp/design-baseline`、`web/temp/v7-baseline`、一次性迁移/退役脚本及其缓存、迁移目录检查临时脚本。实际删除 11 个备份或脚本目标，共 11,124,801 字节；清理前后校验其余 1,686 个 tracked/非忽略 untracked 文件 SHA-256 完全一致，当前 Git HEAD 不变。没有另建备份或转移到其他目录。

统一 OpenSpec 汇入 22 个 active changes、55 份 base specs 与 110 份归档。active change ID 没有冲突；重名 base spec/归档用 server/web 前缀区分。原任务 ID/勾选及历史证据保留，只有文档相对链接调整。模块中的 docs/openspec 是兼容链接，不是重复规范副本。

用户随后明确要求“清理原来的项目”，因此撤销原定一个发布周期的兼容安排，移除三个工作区旧软链接和本地 Git 子模块注册残留。现在只使用新目录；原脚本、个人 IDE 收藏和外部配置若写死旧绝对路径，需要改到新目录。模块内部 docs/openspec/Skills 链接保留。现有 HTTP/MCP/tool/CLI、SDK module 名称、文件引用、操作/权限/存储模型保持不变。

原本依赖本地备份的三仓回滚方式已撤销，不能再使用此前归档中的恢复步骤。当前源码、未提交修改、PRD、active 规格及验收记录继续留在统一仓库；没有删除真实用户数据或修改已部署服务。历史归档记录描述当时状态，不代表这些备份现在仍存在。

此前“前后端分别是 owner”的描述以此次合并决定为准：现在是一个 owner 的 web/server 模块，模块之间依然经公开应用合同交互；外部 Manju、Pinax、Scaena、Eikona 等 owner 不变。产品能力没有因合并删除。开发命令见 [development](development.md)。

迁移和旧入口清理已经本地验收并归档，当前为 21 个 active changes、57 份 base specs。[迁移验证记录](../openspec/changes/archive/2026-09-30-drivebridge-monorepo-consolidation-v1/verification.md) 列出构建、浏览器/MCP 闭环和普通测试结果；[旧入口清理记录](../openspec/changes/archive/2026-09-30-drivebridge-legacy-entry-cleanup-v1/verification.md) 列出目录、配置、MCP 合同和任务保全检查。Server 四处原有失败仍如实保留，未宣称全测试通过。
