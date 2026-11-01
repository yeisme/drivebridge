# 场景矩阵与证据分层

所有新增闭环均为exploratory；不会降级旧功能已经取得的本地证据。规格轮次不运行真实文件传输、模型或部署。

| scenario_id | 用户与工作 | 必需产物 | 检查门 | export/handoff | 验证owner/包 | 晋级信号 |
| --- | --- | --- | --- | --- | --- | --- |
| team-upload-recovery | 素材团队上传文件夹/100GiB文件 | batch、原operation、摘要和耐久分片 | 来源/配额/权限/响应丢失 | 固定FileVersion | Server upload；Client consumers；Local remote | 万项与大文件真实字节恢复，无重复版本 |
| cloud-library-management | 用户浏览三个云盘 | root binding、ExternalEntry、观测代次 | 当前范围、source漂移、cursor失效 | 明确导入的受管副本 | Server cloud-connectors | 各后端独立真机验证，差异如实显示 |
| cross-cloud-migration | 团队迁移材料 | 固定计划、逐项映射、目标摘要 | 双边权限、先验后删、冲突 | 目标固定版本及来源清理回执 | Server cross-storage-transfer | 部分失败只补缺项，取消不误删 |
| analytics-exploration | 运营查看容量/传输/Agent活动 | 同代次事实、授权query、快照 | 聚合权限、数据延迟与口径 | 图表钻取或明确导出 | Server analytics；Client information | 百万文件/千万事件查询目标达标 |
| agent-virtual-organization | Agent整理资料 | labels、collection、plan、policy revision | 风险分级、人工字段、版本前置条件 | 文件集合与逐项回执 | Server organization/agent | 减少人工搬运且无未授权修改 |
| multimodal-evidence-search | 创作者查文档/图片/音视频 | source binding、asset revision、evidence locator | 双边权限、模态/版本/预算 | 固定文件与内容owner证据 | Server content；Manju integration | 四类fixture及各自真实质量门，缺项不伪造 |
| fixed-file-delivery | 团队交付报告/素材集 | 固定清单、holds、接收事实 | 所有必需项可读、明确发布 | 私有交付与分级回执 | Server delivery/retention；Client consumers | 接收方能核对完整内容，业务采纳独立 |
| version-discussion | 团队讨论文件修订 | thread、note、version anchor | 作者与文件权限、并发编辑 | 固定版本讨论 | Server discussion；Client consumers | 双用户/撤权/移动端验收 |
| safe-sync-retention | 用户换机/整理暂存 | 双端计划、baseline、holds | 单writer、冲突双份、删除阈值 | 同步报告和清理预览 | Local sync；Server retention | 中断/挂载刷写/清理竞态均可解释 |
| policy-automation | Agent处理新文件 | outbox、rule、intent、checkpoint | 原ID、去重、独立付费与删除门 | 原文件任务和安全事件 | Server automation；Local remote | 重放不重复修改，失败可暂停与对账 |

每个场景的可运行验证命令在其owner design与tasks中；目前只有 `openspec validate <change-id> --strict --no-interactive` 是本轮执行的规格验证。运行命令必须先实现相应测试入口，禁止no-tests通过。

后续证据位于各owner `temp/integration-test-runs/<run-id>/`，至少summary.json、command.txt、stdout.log、stderr.log、env.json、artifacts/。数据、脚本、硬件、缓存、并发、后端、索引代次必须可追溯，失败不删除证据。

规模目标：1,000,000文件记录、10,000,000事件、10,000文件批次、100GiB真实字节。20并发元数据分页P95≤2秒、常用分析P95≤3秒、内容查询8秒预算、正常负载投影≤30秒；超标必须报告，不能删除慢样本。参考环境见Server验收文档。
