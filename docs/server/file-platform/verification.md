# 文件平台迁移、容量与证据门（待实施）

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


## 本轮状态

能力就绪（TestFPReadiness）与团队运行时（TestFPTeamRuntime）实现与本地定向验证已通过；百万文件预检满规模达标，运行时规模 soak 在单机压缩窗口执行并记录于证据 artifacts。真实文件传输、真实模型调用与发布部署仍未执行，分别报告。分层状态见[能力就绪与运行时状态](runtime-status.md)。

## 规模测试合同

| 维度 | 固定目标/方法 |
| --- | --- |
| 文件与事件 | 单空间1,000,000文件；10,000,000事件，包含重复投递、更新、墓碑和权限变化 |
| 批次 | 10,000文件，含空目录/零字节文件/Unicode路径/冲突/部分失败；分页消费而非全载入 |
| 大文件 | 100GiB真实字节，生成器流式写入并保存已知摘要；实际传输、重启、恢复、读回核验；稀疏空文件或只填size不算通过 |
| 参考环境 | 应用+worker 8vCPU/32GiB；PostgreSQL 8vCPU/32GiB；ClickHouse 8vCPU/32GiB；SSD及独立对象存储，记录网络与可用暂存预算 |
| 元数据 | 20并发，固定查询集预热后至少1000次；P95≤2秒，冷启动单列，不删除慢样本 |
| 分析 | 同并发与数据集，常用过滤/分面/30天趋势P95≤3秒，冷热与每类查询分别报告 |
| 内容 | 沿用Manju8秒预算；返回partial/missing不得算完整成功；质量和延迟分开 |
| 投影 | 正常基准负载100 events/s持续15分钟；P95延迟≤30秒，同时记录积压与追平；峰值另测不外推 |
| 内存 | 单传输工作buffer预算≤256MiB；进程RSS、并发与baseline分别记录，100GiB不整文件入内存 |
| 吞吐 | 报有效字节/总耗时及验证/暂存开销，与同环境后端基线对照；不承诺跨网络固定速率 |

这些是验收目标，不是性能结果。后端自身限制、空间配额、策略和安全预算取交集；当前默认上限不因文档改变。

## 定向与最终验证

各change tasks先增加TestFP主题入口再执行其命令；缺少测试、skipped或no tests to run不算通过。复用现有Go testing、testscript、Vitest/Testing Library和Playwright；不新建并行框架。

Server现有入口：

```bash
python3 scripts/check.py -- go test ./internal/service -count=1
python3 scripts/check.py -- go test -race ./internal/service -count=1
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge-server
```

实现中仅运行所改包定向用例；稳定后才跑最终门。100GiB等长测需要先在现有runner增加显式可配置timeout和逐步日志落盘；保留原默认900秒、不吞超时前输出，不另起框架。真实云测试成本与外部写入另行授权。

证据位于本owner temp/integration-test-runs/<run-id>/，六件套为summary.json、command.txt、stdout.log、stderr.log、env.json、artifacts/。数据规模、查询种子、软件版本、硬件、网络、冷热缓存、分位数、失败率和摘要均由runner生成。不能把私有文件正文、SQL参数、原始模型请求或凭据放入证据。

## SQLite到PostgreSQL

停写并确认旧writer隔离→CLI导出→导入新库→核对实例/文件/版本/操作/权限/配额/独立密钥引用→只读比对→切换写入。对象字节不移动。新写入前可回原库；新写入后必须反向迁移或向前修复，禁止回旧快照造成丢失。

## 运行时故障门

覆盖响应丢失、重启、lease旧writer、源变化、配额耗尽、撤权、批次部分成功、索引乱序/重复/墓碑、ClickHouse宕机、DuckDB缺失、Manju缺模态、交付缺项、purge/hold竞态和补偿冲突。

当前只生成设计，不执行迁移。验收完成以每个owner的真实证据为准；文档与spec通过不会自动勾选实现任务。
