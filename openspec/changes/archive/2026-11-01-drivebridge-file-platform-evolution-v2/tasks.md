# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。

- [x] 1.1 设计基线与保留能力；核对现有源码、十四项required能力和数据分层决定；验收handoffs每项唯一owner/spec/tasks，失败回查当前源码和用户范围。 | evidence: verification.md; 19 change strict passed; implementation tasks remain unchecked; root identity archival is concurrent
- [x] 1.2 跨项目合同与依赖；固定File/Version/Operation/SourceBinding及双边权限，补Mermaid与Anatomia/Sonora边界；验收design/spec/scenarios一致，失败不以模糊owner交接。 | evidence: verification.md; 19 change strict passed; implementation tasks remain unchecked; root identity archival is concurrent
- [x] 2.1 owner规格交接；通过openspec CLI建立主包与配套包，脚本生成未勾选实施任务；验收六组任务、真实命令和证据位置齐全，不覆盖Manju原任务。 | evidence: verification.md; 19 change strict passed; implementation tasks remain unchecked; root identity archival is concurrent
- [x] 2.2 文档同步；更新三个DriveBridge owner文档、Manju联调入口与根导航，纠正本机只读/交接阶段旧描述；验收当前/计划/历史证据不混写。 | evidence: verification.md; 19 change strict passed; implementation tasks remain unchecked; root identity archival is concurrent
- [x] 3.1 规格治理验证；逐包openspec validate --strict --no-interactive，检查Markdown链接、依赖环、接口命名、任务状态与git diff --check；仅关闭设计门。 | evidence: verification.md; 19 change strict passed; implementation tasks remain unchecked; root identity archival is concurrent
- [x] 3.2 交接复核；对比原任务哈希和dirty基线，确认无产品代码、运行配置、数据迁移、真实服务或发布动作；报告规格证据和未执行的运行门。 | evidence: verification.md; 19 change strict passed; implementation tasks remain unchecked; root identity archival is concurrent
