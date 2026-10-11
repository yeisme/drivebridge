# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。任务分组：A=绑定与身份，B=grant 生命周期合同，C=受控读取与发现，D=回存，V=全量验证，F=文档与收尾。串行 lane 说明：A→B→C→D 存在状态依赖，必须按序；V 依赖 1.1–1.4；F 最后。本 change 不调用 submitCreative/startCreativeSaveback，不运行在途 creative.go/creative_save.go 的生成提交/轮询路径，不运行真实网盘或生成 Provider；真实三 owner 组合和部署由根验收另报（单独授权/单独报告）。

## 1. 绑定配置与接收身份（Requirement: 接收身份与目录绑定）

- [x] 1.1 依赖无；串行 A。实现用户级绑定配置/inspect/disable 入口及 help（配置/凭据经现有 CLI/application service 写入，只引用用户私有 credential store，不要求手写 JSON/YAML），可信 scoped Agent 与 consumer/project/purpose/目录映射，旧权限不扩张，Agent 不能自行增加绑定或签发权限 | owner: `internal/service/mcp.go`、`cmd/drivebridge-server/commands.go`、`internal/config` | 验证： `python3 scripts/check.py -- go test ./internal/service ./sdk -count=1`，覆盖伪造身份、tenant/space 和目录越界；预期： 伪造 consumer/project 或文件移出授权前缀被拒绝，不返回跨范围字节；失败复查： 按原键复查绑定记录与 share 规则，不改身份语义。 | evidence: Owner binding, missing-agent rejection and scoped receiver tests; 20260916T134131Z-cbf92762.

## 2. 幂等绑定与 grant 生命周期（Requirement: 幂等绑定与可恢复读取）

- [x] 1.2 依赖 1.1；串行 A。以 GORM 唯一键（data owner+认证主体+逻辑幂等键）与事务实现 Bind/Get/Renew/Cancel（HTTP：`POST /api/v1/handoffs`、`GET /api/v1/handoffs/{id}`、`.../renew`、`.../cancel`；SDK：BindReference/GetHandoff/RenewHandoff/CancelHandoff），旧 MCP input.bind_reference 委派到同一 service 并补齐幂等键语义；同指纹返回原 grant 不延长有效期，异指纹返回 `idempotency_conflict`；新增可选 `consumer_request_ref`/`request_expires_at`，续取不延长请求期限、取消幂等且终态不可续取，Consumed 不因单次读取自动置位 | owner: `internal/service/handoffs.go`、`internal/service/http.go`、`internal/service/mcp.go`、`sdk` | 验证： `python3 scripts/check.py -- go test ./internal/service ./sdk -count=1` 覆盖并发同键、异指纹冲突、取消与到期；预期： 并发重放只有一个 grant，原 grant 不被重建；失败复查： 检查唯一约束冲突路径而非单进程锁。 | evidence: Concurrent same-key grant reuse, conflicting fingerprint, renew/cancel; 20260916T135743Z-d61e989b.

## 3. 受控内容读取与能力发现（Requirement: 固定版本的受控传输）

- [x] 1.3 依赖 1.2；串行 A。实现 OpenHandoff 经同一已认证连接的固定版本流式读取（HTTP `GET /api/v1/handoffs/{id}/content`），每次重查身份、scope、share 规则、文件版本与当前目录，关闭重定向；grant ID 不是 bearer 权限；不返回第三方签名 URL，不把临时 URL/凭据/字节正文放入普通 MCP 输出；能力发现增量返回已实现操作、当前主体可执行性与阻塞原因 | owner: `internal/service/handoffs.go`、`internal/service/http.go`、`internal/service/discovery.go`、`sdk` | 验证： `python3 scripts/check.py -- go test ./internal/service ./sdk -count=1` 覆盖内容替换、撤销、目录移动、续取及无临时 URL 输出；预期： 固定内容不存在或权限撤销时拒绝读取、不降级 latest；失败复查： 保留原记录，先核 share/目录判定再核流传输。 | evidence: Pinned old bytes after replacement, directory move rejection and binding disable; 20260916T135743Z-d61e989b.

## 4. 来源项目回存（Requirement: 回存文件回执不代表领域完成 / 在途 creative 与旧合同隔离）

- [x] 1.4 依赖 1.3；串行 A。复用现有 Upload/Download、版本、配额与不可默认覆盖规则实现逐项回存：上传键绑定源 owner/project/version/item、目标 space/path 与摘要，重复项复用原 operation/file version，换目标或内容换逻辑操作；最小来源清单作为受控上传对象最后上传；返回安全 file/version/digest/operation 回执；Server 不解释选定审核、不启动生成、不替来源项目建 outbox，不以 `CreativeReceipt.Status=archived` 推断回存分组完整 | owner: `internal/service/files.go`、`internal/service/handoffs.go`、`sdk` | 验证： `python3 scripts/check.py -- go test ./internal/service ./sdk -count=1` 覆盖重复项、配额、清单失败、旧 creative 状态不变；预期： 文件成功但清单失败时保留成功项与原 operation，不报告整个交付分组完成；失败复查： 对照 creative_test.go 确认在途状态语义未变。 | evidence: Existing upload/Download plus Eikona manifest-last real-binary system scenario system-20260916T134601.407859518Z; no generation integration.

## 5. 全量验证与证据

- [x] 1.5 依赖 1.1–1.4；串行 V。运行 `python3 scripts/check.py -- go test -race ./internal/service ./sdk -count=1`，验证真实 HTTP/DB、服务重启后 grant 与内容保持、旧 input.bind_reference 返回形状及其他 creative 路由回归；检查测试输出确实包含新增用例（拒绝 no tests to run）；证据归 `temp/integration-test-runs/<run-id>/` 六件套，由已有 runner 创建，失败保持原退出码并脱敏 | owner: `internal/service`、`sdk` | 预期： race 全绿、证据六件套齐全且无凭据/临时 URL；失败复查： 归因后回对应任务，不删用例。 | evidence: Full service/SDK/CLI race 20260916T134131Z-cbf92762; explicit service restart preserves original grant and contents 20260916T135743Z-d61e989b.

## 6. 文档与收尾

- [x] 1.6 依赖 1.5；串行 D。同步 `docs/creative-resource-handoff.md`（本地合同、MCP-only/配置/恢复说明）；运行 `go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge-server`、`openspec validate drivebridge-creative-handoff-v1 --strict --no-interactive`；记录实际命令与证据，不做真实账号/部署声明；本包仅为实施规格 closeout，客户端与三个创作 owner 的实现/部署不属于本项目本地任务 | owner: 全项目 | 预期： 三命令全绿、strict 通过、文档与实现一致；失败复查： 修源因后重跑整条。 | evidence: docs/creative-resource-handoff.md synchronized; go vet, CGO_ENABLED=0 build and strict OpenSpec validation passed; no deployment.
