# 本机CLI/MCP演进与Server合同消费（设计）

## 当前行为

本机负责允许root中的文件、稳定引用、本机操作、同步和挂载；Server负责服务端文件状态与云端管理。当前已有relay、原Operation恢复、CAS、受控本机文件操作与Pinax路径挂载代码；源码/归档证据不替代真实云或跨平台验收。

`drivebridge view`监听loopback，本机网页通过私有owner socket调用已启用动作，不打开第二个可写数据库。owner缺席时展示只读索引；不是任何情况下“永远只读”，也不是远程可写文件服务。

## 规划入口

- [本机同步与冲突保护](../openspec/changes/drivebridge-sync-retention-v2/design.md)：计划、双端复核、单writer、删除阈值、VFS刷写与分类清理。
- [远程CLI/MCP消费](../openspec/changes/drivebridge-remote-surfaces-v2/design.md)：批次、查询、整理、快照及原Server ID恢复。该包已本地实现（默认关闭，见下节）。

当前发现命令：

```bash
drivebridge capabilities --json
drivebridge relay targets --json
drivebridge mount status --json
```

## 远程合同消费（已实现，默认关闭）

`drivebridge-remote-surfaces-v2` 的应用合同落在 `internal/bridge/remote_surfaces.go`：冻结22个公开动作（upload_batch_*、analytics_*、organize_*、cloud_*、changes_v2、change_subscription_*、automation_intent_get），CLI/MCP/字节路径同源；意图日志 `RemoteIntent` 加法进入本机 state。人类专属管理面（OAuth连接发起、计划确认、整理策略、自动化规则）不注册给Agent。

新入口默认关闭，须显式 `--remote-surfaces` 且以Server实时 capabilities 广告为准注册；未广告动作返回 `unsupported` 并列出实际缺口。可用命令：

```bash
drivebridge remote --action capabilities --remote-url <url> --remote-token-file <file> --local-root <dir>
drivebridge remote --action upload_batch_get --remote-surfaces --remote-url <url> --remote-token-file <file> < args.json
drivebridge remote --action upload_batch_file --remote-surfaces --remote-url <url> --remote-token-file <file> --local-root <dir> < args.json
drivebridge mcp --remote-surfaces --remote-url <url> --remote-token-file <file> --local-root <dir>
```

回滚演练：去掉 `--remote-surfaces` 即恢复旧行为（tools/list 与旧客户端逐字节一致，已由测试固定）；删除 state 中的 `remote_intents` 表只丢本机意图日志，不丢Server侧真值（已由迁移测试固定）。响应丢失恢复用 `operation_lookup` 原键原请求重放，原Server ID原样保留。与真实后端（backend-server 五个Wave 2包）的联调为 INTEGRATION_PENDING，当前验证全部来自合同 fixture，证据见本owner temp/integration-test-runs/ 对应 run。

## 无CLI客户端

先读取已连接MCP的tools/list、inputSchema和capabilities；通过HTTP或网页传字节，MCP只传安全ref。纯远程MCP不读取客户端路径。没有本机adapter时给出既有上传入口，不能自动安装工具、扩大root或启动挂载。

operation_id属于返回它的实例。远程失败不自动转本机rclone、其他账号或另一个存储目标；恢复先查原ID。同名文件不等于原内容，源摘要变化必须拒绝。

## DuckDB快照

仅消费Server拥有的共享typed分析适配与显式授权快照格式；不复制远程查询规则，不读取Server私有数据库。可选DuckDB进程不链接进主二进制、无网络/扩展自动安装/自由SQL；缺依赖如实报告（`analytics_backend_unavailable`，不影响上传与服务端查询）。本机分析的是导出副本，不能声称撤权能收回已导出的副本。

## 同步和保留

现有sync是用户终端入口，MCP不新增apply；首次resync的本地优先必须说明。计划执行前核对两端变化，默认25%删除阈值保留；冲突保留双份。仅本机锁不能证明其他机器已停止，跨机接管要求明确交接。

缓存、未刷写数据、上传暂存、回收站和Server版本分别归属，挂载alive不等于远端保存。文件夹软删除不变成purge；保护路径规则继续有效。

`sync plan/apply/status` 与 `retention preview` 已实现并通过本地 fixture 验证（见 [本机文件操作](local-file-experience.md)）；真实后端联调未做。

## 验收与规模

新增TestFPSyncPlan和TestFPRemoteSurfaces后才执行各design定向命令；空测试不算通过。现有可运行基线：

```bash
python3 scripts/check.py -- go test ./internal/bridge ./internal/remote -run TestFPRemoteSurfaces -count=1
python3 scripts/check.py -- go test ./internal/bridge -run 'Test(Sync|Retention|DownloadCAS|PrepareSpace|RcloneMount)' -count=1
python3 scripts/check.py -- python3 scripts/remote_system.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge
```

规模入口已本地命中：万项批次按200/页keyset分页全量读回（50页、原ID逐项保留）；100GiB流式入口经真实HTTP分片（256MiB片、400片、单缓冲复用、累计分配远低于流经体积），runner显式timeout=900s且失败保留日志。不将现有本机1000文件目录作业上限直接提高。真实云与真实后端不重复计作本机证据；平台ACL、socket与挂载依赖仍需实机验收。证据归本owner temp/integration-test-runs/<run-id>/六件套。
