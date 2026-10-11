# 单仓迁移验证（2026-09-30）

本次完成本地仓库与开发工作流合并，没有执行前端重建、真实云操作、发布或推送。三个模块的 `git rev-parse --show-toplevel` 均为 `cli/drivebridge`。

| 验证层 | 结果 | 本 owner 证据 |
| --- | --- | --- |
| 文件迁移 | 1847 个 tracked/非忽略 untracked 文件移动后摘要一致，原 Git 元数据保留 | `.git/consolidation-backup/migration.json`（本地私有） |
| CLI / Server / Web build | 全部通过，Web 保留既有大 bundle 提示 | `temp/integration-test-runs/20260930T114658Z-32b0be17/`、`20260930T114703Z-282718ec/`、`20260930T114707Z-0ae8f589/` |
| CLI 普通测试 | 通过 | `20260930T114921Z-4824a02e` |
| Web Vitest | 38 文件、224 项通过 | `20260930T115637Z-58e3f453` |
| 浏览器现有主流程 | 3 项通过：登录/上传/版本/Agent隔离、原操作恢复、来源与固定引用 | `20260930T115441Z-931e0e2c` |
| 统一 dev 实际浏览器 | 启动→体验登录→上传→owner 回读通过，无 pageerror；停止后两端口关闭 | `20260930T115526Z-d5415341`；退出检查记录于本次终端 |
| CLI/MCP/HTTP fixture | 上传/下载/幂等/目录边界/撤权全部通过 | `20260930T115825Z-61513fe1` |
| Go vet | CLI 和 Server 均通过 | `20260930T115127Z-15e92125`、`20260930T115324Z-fc8a700c` |
| 开发编排器 | 5 项通过，含端口占用不杀旧进程、继承 race 拒绝、失败退出码、进程组退出、继续执行剩余模块测试 | `20260930T115825Z-d5436fbc` |
| OpenSpec | 22 active changes + 55 base specs，77/77 strict 通过（迁移包归档前） | `20260930T120041Z-46f3ece9` |
| 文档 / Profile / 差异 | 当前文档及 active change 的 547 条相对链接通过；统一 profile 有效；本轮 diff 检查通过 | 本次命令回读；后续新增收尾链接单独复核 |

## 已有失败，不属于迁移引入

Server 全量测试保留四处失败，`make test` 会如实返回非零；聚合入口仍会继续收集 Web 测试结果：

- `TestEncryptedSpaceRoundtripAndCiphertextAtRest`：解密流字节不符。
- `TestGovernanceTenantApprovalAndRevocation`：批准写入返回 403。
- `TestGovernanceEmergencyAndManualRecovery`：紧急恢复写入返回 403。
- `TestGovernanceExactOverwriteExportAndEnrollment`：覆盖项返回 approval_required。

完整运行证据：`20260930T115138Z-aacb86c3`。从迁移前摘要确认的 182 个原始 Server 业务文件重建隔离基线，同四项逐一复现：`20260930T115636Z-9707335b`。未修改加密或治理业务以清门，不能宣称全仓测试全绿。

原 remote_system.py 测试脚本仍从 stderr 查错误字符串，而当前 `--json` 合同已在 stdout 返回 `error.code`。本次修正脚本为精确校验 JSON 中的 remote_unsupported，保留所有权限/恢复反例，不改业务合同；复跑通过。

## 保留和限制

旧工作目录是兼容链接，Git 历史备份在本机；远端仓库还没有合并发布。前后端 62 项产品任务的 ID 和勾选状态未改变。未运行 race、真实账户、付费模型、原生设备或生产部署验证。

归档后补验：21 个 active changes 与 56 个 base specs 共 77 项再次通过 strict（`20260930T120548Z-b7544454`）。最终目录/兼容链接/统一 Skill 与任务状态检查通过，本项目 567 条相对链接无缺失（`20260930T120824Z-d9dafd47`）。根架构文档另有 11 条未由本次改动的其他项目历史断链，已分开记录，不将其描述为全工作区链接通过。
