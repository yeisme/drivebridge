# 能力就绪与团队运行时：实现状态、真实命令与回滚演练

本文只记录已落地的源码事实与本地验证证据；真实后端联调、真实云存储与发布部署仍是独立事实，不由本文代替。规格真源：[动作能力就绪](../../../openspec/changes/drivebridge-capability-readiness-v2/design.md)与[团队运行时](../../../openspec/changes/drivebridge-team-runtime-projections-v1/design.md)。

## 本轮工作台设计适用边界

2026-10-01 只读确认 `internal/service/model.go` 的 PostgreSQL adapter 与 `cmd/drivebridge-server/metadata.go` 迁移命令已存在。下表与后文规模数字是原记录，不是本次重新验证；新工作台只在未来实施时以 SQLite 验首条闭环，PostgreSQL 真实兼容/迁移/规模仍独立验收。选型见 [技术决定](../../architecture/personal-workspace-technical-decisions.md)。

## 历史能力状态分层

| 层 | 状态 | 证据 |
| --- | --- | --- |
| 规格与 strict 校验 | 通过 | `openspec validate drivebridge-capability-readiness-v2 --strict --no-interactive`、`openspec validate drivebridge-team-runtime-projections-v1 --strict --no-interactive` |
| 本地实现与定向测试 | 通过 | `python3 scripts/check.py -- go test ./internal/service -run TestFPReadiness -count=1`、`python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1`，六件套位于 `temp/integration-test-runs/` |
| 百万文件预检规模 | 达标 | `DRIVEBRIDGE_FP_SCALE=full` 下 1,000,000 文件 fixture、10,000 次预检、20 并发、P95 7ms、最大 20ms |
| 运行时规模 soak | 本机压缩窗口 | `DRIVEBRIDGE_FP_SCALE=full` 下事件量与乱序重放达标、发布无重复；15 分钟窗口在本机压缩执行，结果写入证据 artifacts |
| 真实后端 / 真实存储 / 发布 | 未执行 | 未获授权；保持未完成，不以 strict 或本地证据冒充 |

## 已实现公开合同（本 owner）

- `GET /api/v1/capabilities?space_id=ID`：旧键全部保留，新增 `actions`（已实现动作清单）与 `limits`（有效上限交集；未知值为 null）。
- `POST /api/v1/capabilities/check`：只读预检，输入 `action`、`space_id`、可选 `ref`；返回 `ActionReadiness`（`implemented/configured/authorized/dependency_status/connection_observed_at/availability/block_reason/recovery/limits`）。未知动作返回 `unsupported_action`（422），依赖故障返回 `dependency_unavailable`（503）。
- MCP：`capability_check` 工具与 HTTP/SDK 调用同一应用裁决；`capabilities` 工具响应加法扩展。观测与配置分离：`connection_observed_at` 为空即 `unknown`，预检自身永不探测连接。
- `GET /api/v1/runtime/status`：operator 只读运行时视图（队列、worker 租约、投影水位），不返回 DSN。MCP `runtime_status` 同一裁决，非 operator 拒绝。
- `POST /api/v1/projections/{name}/rebuild`：operator 发起新代次，返回原 `operation_id`；不向普通 Agent 注册。切换前 `projection_not_caught_up`（409）拒绝；校验数量/摘要/墓碑一致后原子切换读指针。
- `drivebridge-server metadata export/import/verify`：静默（quiesce）元数据迁移 CLI；PostgreSQL 目标 DSN 只从 `DRIVEBRIDGE_TEAM_DSN` 环境读取，永不作为命令参数或输出。新写入后回旧库返回 `metadata_migration_requires_quiescence`（409），必须反向迁移或向前修复。

## 真实命令

以下为用户实际执行的命令（无本地包装器）：

```bash
cd cli/drivebridge/server
python3 scripts/check.py -- go test ./internal/service -run TestFPReadiness -count=1
python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntime -count=1
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPReadinessScalePreflight -count=1 -v
DRIVEBRIDGE_FP_SCALE=full python3 scripts/check.py -- go test ./internal/service -run TestFPTeamRuntimeScaleSoak -count=1 -v
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
cd ..
openspec validate drivebridge-capability-readiness-v2 --strict --no-interactive
openspec validate drivebridge-team-runtime-projections-v1 --strict --no-interactive
```

迁移操作（operator，离线）：

```bash
drivebridge-server metadata export --out snapshot.jsonl
DRIVEBRIDGE_TEAM_DSN='postgres://...' drivebridge-server metadata import --source snapshot.jsonl
drivebridge-server metadata verify --left snapshot.jsonl --right target-snapshot.jsonl
```

Race 检测按 [AGENTS.md](../../../AGENTS.md) 禁令不运行；证据 summary.json 记录 `race skipped per AGENTS.md`。

## 回滚演练

1. **能力就绪加法列**：`connections.observed_at/observed_status` 为可空列。默认关闭新入口并保留加法列，从未探测的连接仍为 unknown；不把删除列作为自动回滚。
2. **团队运行时新表**：默认保留 `runtime_jobs/runtime_outbox/runtime_leases/runtime_checkpoints/runtime_generations/runtime_projection_rows/runtime_sequences`，停新增任务后核查原操作和待发布状态并向前修复；不能默认 drop 状态表丢弃恢复身份。真实清理或反向迁移须单独授权。有效新 restriction profile 凭据不得交由不认识它的旧 binary 鉴权。
3. **投影代次**：旧代次行保留作诊断；如需回退读指针，通过一次新的 rebuild（代次前进）完成，不做跨代次混读。
4. **元数据迁移**：切换写入前可回原库；产生新写入后禁止回旧快照（`metadata_migration_requires_quiescence`），只能反向迁移或向前修复。实例密钥独立于快照搬运，缺失密钥的库以 `instance_key_missing_restore_required` 拒绝启动，不静默重造。

### 受控自动化实施状态（已实现，本地验证）

受控自动化面已落地并由 `TestFPAutomation*` 本地验证（fixture，离线）：

- 合同：`GET /api/v1/changes`（既有 outbox 之上的安全引用事件页：event_id/resource_ref/revision/kind/origin，无名称/正文/临时URL；AEAD 游标绑定 owner+sequence，过期 `cursor_expired` 需受控重建；至少一次投递由消费方按 event_id 去重）、`POST /api/v1/change-subscriptions`（授权范围内认证轮询消费者，无任意 webhook）、`POST /{id}/ack`（单调 checkpoint，越界 `ack_ahead`）、`GET/PUT /api/v1/spaces/{id}/automation-rules`（人类管理：动作/前缀/预算/有效期/revision/清除暂停；Agent 只读）、`POST /api/v1/automation-rules/{id}/run`、`GET /api/v1/automation-intents/{id}`（原rule/event/operation与暂停原因）。
- 语义：意图唯一键 (event_id, rule_revision, actor, scope)——重放落原意图；三类同类确定失败暂停规则（`rule_paused`）并由有权主体 clear_pause 恢复；预授权失效 `policy_revoked` 停止新动作保留完成回执；标签回写不产生文件事件，规则不自触发。
- 附带修复：blob-reuse 即时完成路径现在同样写入 file_catalogue 事件（Wave1 "committed file never missing from outbox" 不变量补全，TestFPTeamRuntime 全组回归通过）。
- 数据模型：`change_subscriptions`/`automation_rules`/`automation_intents` 纯新增表。

真实命令（本地 fixture 验证入口）：

```bash
cd cli/drivebridge
python3 scripts/check.py -- go -C server test ./internal/service -run TestFPAutomation -count=1 -timeout 5m
```

留白：1000万事件含重复乱序、100events/s 持续15分钟与暂停重启规模未本机验证；本机自动化消费适配包归本机 owner。
