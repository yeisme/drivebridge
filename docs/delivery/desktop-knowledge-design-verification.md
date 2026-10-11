# 桌面知识阅读设计交付核验

2026-10-01。本记录只说明本轮规格/文档检查，不是API、Electron、数据库、安装包或实机通过证明。产品实现任务保持待办，未安装依赖、启动产品服务、运行产品测试/构建/race、访问真实vault、同步、发布或部署；没有提交或推送。

## 交付

- 根交接：`drivebridge-pinax-reading-integration-v1`，治理与owner，不持业务实现进度。
- DriveBridge：`drivebridge-desktop-knowledge-reader-v1`，15项后续任务；[Electron](../architecture/electron-desktop.md)、[三屏UI](../web/ui/knowledge-reading-spec.md)、[验收矩阵](desktop-knowledge-acceptance.md)。
- Pinax：`pinax-human-reading-api-v1`，9项后续任务；[人类阅读合同](../../../pinax/docs/interfaces/human-reading-api.md)。
- 三个包合计7个delta spec、31个Requirement、32个Scenario。原74条F/B复用和后续写入交接明确，未复制reader或新增独立客户端。

## 已执行检查

| 检查 | 结果与边界 |
| --- | --- |
| 六个change strict validate | 全部通过：根新包、DriveBridge新包/原rebuild/原contracts/旧mount、Pinax新包；不是业务测试 |
| 相对链接与新增锚点 | 新增链接/锚点无失败；既有根产品边界文档有11处历史失效链接，归pre-existing，未越界修复 |
| 空白与diff检查 | 本轮文档无trailing whitespace，三个owner相关路径 `git diff --check` 通过；对未跟踪新文档另做行尾检查 |
| 新任务阶段/依赖 | 24条子项目任务各只有一个PHASE；具名直接依赖存在，新任务依赖图无环；后续能力任务保留 |
| 原F/B保护 | 原44条前端+30条后端task ID、勾选与原文字前缀全部保留，只通过amend追加关联 |
| 原Pinax真实验收 | 五份tasks.md与本轮开始前SHA-256一致，仍11项未完成；没有重置continuity窗口 |
| Electron官方依据 | GET核验官方security/context isolation/sandbox/packaging；固定参考commit见Electron架构；未安装依赖或测试所选精确版本 |
| 修改范围 | 仅治理、OpenSpec、文档与CLI维护的任务metadata；未创建desktop产品目录或修改产品源码 |

当前工作区有大量预先存在且并发变化的dirty工作。根AGENTS.md的Radar退役段在本轮期间被其他工作更新，归concurrent并完整保留，不计入本次阅读交付；没有用整文件恢复覆盖它。

## 可复核命令

仓库根：

```bash
openspec validate drivebridge-pinax-reading-integration-v1 --strict --no-interactive
git diff --check -- AGENTS.md docs/architecture/subproject-product-boundaries.md openspec/changes/drivebridge-pinax-reading-integration-v1
```

`cli/drivebridge`：

```bash
openspec validate drivebridge-desktop-knowledge-reader-v1 --strict --no-interactive
openspec validate drivebridge-personal-workspace-rebuild-v1 --strict --no-interactive
openspec validate drivebridge-personal-workspace-contracts-v1 --strict --no-interactive
openspec validate drivebridge-pinax-vault-mount-v1 --strict --no-interactive
git diff --check -- AGENTS.md docs openspec/changes
```

`cli/pinax`：

```bash
openspec validate pinax-human-reading-api-v1 --strict --no-interactive
git diff --check -- AGENTS.md docs openspec/changes/pinax-human-reading-api-v1
```

机器schema/TS生成、reader endpoints、session flags、新evidence profiles、desktop命令均仍是planned。三平台实机、签名/公证/公开分发、两周自然使用各自未验；本记录不可用来勾选D/P实现任务或原Pinax真实任务。
