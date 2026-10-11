# 本轮本地验证

## Race 检测执行限制

Agent 在任何阶段都不得主动运行 race 检测，也不得通过子 Agent、Skill、CI、验收、发布、聚合任务或脚本间接触发。只有当前人类用户明确、强烈要求执行 race 时，才允许在指定范围内运行一次；失败、超时、中断均不得自动重试，后续运行必须取得新的同等明确要求。一般的“测试、验收、修复、发布、跑全部检查”不构成授权。

Race 不属于默认验收或完成门禁；未运行不得阻塞交付，也不得声称已经通过。执行聚合命令前检查其依赖，移除或跳过 race 阶段，继续普通测试、静态检查和构建。本规则取代本文件及 Skills、OpenSpec、历史计划、文档中的自动或强制 race 要求；它们不能作为执行授权。


本轮为独立本地候选，没有真实 OIDC/OneDrive/S3 账号操作、付费生成、公网部署或发布。

## 自托管激活真机验收（2026-09-16）

首个真实部署在内网长期服务器激活：key-only（无 OIDC）、仅平台 S3、Caddy 内部 CA 做 TLS 终结（serve 保持回环，origin 为 https 内网地址）。

- 离线彩排先行：`temp/integration-test-runs/20260916T072125Z-79c833f7/`（四步序列 + 浏览器等价冒烟 + 备份只读校验全绿，明文 key 仅在 0600 artifacts 分发文件）。
- 真实部署：真实腾讯 COS（既有授权桶 `drivebridge-1384001538`、前缀 `drivebridge`、virtual-hosted 寻址）；`platform-connect --verify-isolation` 如实报 `over_scoped`（root 桶凭据，缓解路径：CAM 前缀子凭据或空间信封加密）。
- 部署形态决策修正：serve 拒绝非回环 http origin（auth 合同），因此 TLS 终结是内网激活的硬前提；Caddy `tls internal` + `auto_https disable_redirects`（避开 :80 冲突），凭据只经 rclone 配置管道进 `platform-connect` stdin，临时文件即毁。
- 首验门全过：auth/mode 如实 key-only、用户 key 换会话、平台存储建空间、上传/下载字节一致、幂等重放、caller 溯源、配额三值、`status --url` 远程核对（admin 与普通用户空间互不可见）、停服 backup + `doctor --backup` ready。
- 遗留：操作者本人设备的浏览器信任 CA 后实测；instance.key 独立渠道保管；重启持久化目前为 start.sh 手动拉起（未配 systemd）；token 文件严格要求单行无换行（CRLF/尾换行会被拒，属使用锐角）。
- 部署细节（内网地址、密钥分发文件）不落入仓库，留在部署机 `~/drivebridge/README.md`。

- Go 全量/race（OIDC、S3 HTTP、OneDrive 合同、权限、操作恢复、取消及备份）：`temp/integration-test-runs/20260911T073927Z-c97c9ee5/`。
- Go vet：`temp/integration-test-runs/20260911T073937Z-b2b87438/`。
- CGO_ENABLED=0 构建：`temp/integration-test-runs/20260911T073938Z-497c4ed6/`。
- 本 owner OpenSpec 严格校验通过。

验证使用两个独立用户、真实 RSA OIDC 签名/PKCE、真实 Go S3 SDK 的本机 HTTP fixture、原生 Streamable HTTP MCP；OneDrive 使用受控 Graph/传输契约 fixture，不声称微软账号链路已验收。

已验证目录范围、只读/过期/撤销 token、跨用户文件/版本/操作/连接拒绝、重复上传/完成、分片替换、存储断开恢复、配额预留、版本冲突、软删除、元数据与密钥恢复、生产 cookie 边界、取消活动云传输。备份缺失密钥或实例 ID 会拒绝启动；缺少本机分片会要求原文件重传。

SDK/MCP 敏感交接检查的初次断言把普通状态文字里的 `http` 也当作 URL，随后改为检查实际 URL 协议和签名参数，保持敏感字段断言；失败证据保留。

未把此前 Scaena/Eikona 的在途改动纳入本轮修复，也未改写既有本机 DriveBridge 的历史回执。

## 新租户四步真机开箱验收（2026-09-16）

真实腾讯云 COS（用户授权桶）端到端：config init → platform-connect（verify-isolation 如实报 over_scoped）→ users import（自生成+外部导入+admin 混批）→ key-only serve → 六门验收（key 换发会话/外部 key 直连/建空间/MCP 协议 26 工具/远程 CLI 上传/rotate 会话级联）→ COS `<instance>/<user>/<space>/<version>` 命名空间实见后清理。报告：`temp/integration-test-runs/20260916-tenant-bootstrap-acceptance/report.md`。遗留：rotate 无 HTTP 端点（CLI 与 serve 互斥 state 锁时需停服窗口）。

## deployment-readiness（2026-09-11）

- Go 全量/race（含 doctor 单测）：`temp/integration-test-runs/20260911T144509Z-6c7233af/`。
- go vet：`temp/integration-test-runs/20260911T144513Z-c7aa379a/`。
- CGO_ENABLED=0 构建：`temp/integration-test-runs/20260911T144513Z-0db84e26/`。

doctor 预检只读：不取实例锁、不调用 `Open`（避开 interrupted 改标与 repairParts）、`mode=ro` 读库；密钥缺失而库存在时给恢复指引而非重建暗示；issuer 探测只验证 discovery 文档与 issuer 字段一致，不做账号级验收。emit 增加 data 参数（platform-connect/backup 传 nil 时输出不变），`--agent` 补标量 `fact.*`。生产部署清单见 [deployment.md](deployment.md)。

## personal-agent-loop-v1（2026-09-12）

- 全量 Go/race（含来源、历史版本解析与权限）：`temp/integration-test-runs/20260912T161936Z-e284449b/`。
- Go vet：`temp/integration-test-runs/20260912T162054Z-d0158a33/`。
- CGO_ENABLED=0 服务端构建：`temp/integration-test-runs/20260912T162340Z-9d8cd649/`。
- 本 owner OpenSpec 严格校验通过。

SDK 上传增加可选来源字段，省略时保持原请求指纹。跨用户固定引用返回拒绝；同一文件新版本不改变旧引用的内容和来源。仅本地 fixture，无真实账号、部署或付费服务。

## 2026-09-16 多租户治理与审批

- 全项目 `go test -race ./...` 通过：`temp/integration-test-runs/20260916T111813Z-cca2be3d/`；`go vet ./...` 与 `CGO_ENABLED=0 go build ./cmd/drivebridge-server` 通过。
- 授权自动失效通知的补充检查通过：`temp/integration-test-runs/20260916T112144Z-d348fbd7/`。
- 新增验证覆盖独立租户与个人空间、team 管理员隔离、自批／自授拒绝、范围许可撤销、精确覆盖与版本竞争、ZIP 导出、人工恢复暂停特权、激活码一次性、旧密钥旁路拒绝、审计故障、凭据撤销中断流，以及存储 I/O 期间撤销不阻塞且不复活请求。
- OpenSpec `drivebridge-multitenant-governance-v1` 通过验证；以显式部署侧引导激活，新角色不从旧 admin 自动继承。未执行真实启用、云账号访问、部署或发布。

## 2026-09-17 内网预览默认对接腾讯 COS 存储验证

- 场景：本机内网预览（key-only serve，`--state temp/lan-preview/state`，`--web-dir` 托管 client dist，监听 `0.0.0.0:48091`）通过 `platform-connect --verify-isolation --json` 接入既有授权桶（区域 ap-guangzhou，前缀 `drivebridge`，virtual-hosted 寻址）。凭据仅经 stdin 从本机用户级 rclone 配置管道传入，未落任何仓库文件；连接加密封存于 state（由 `state/instance.key` 保护）。
- platform-connect 结果 `partial`：`storage_isolation=over_scoped`（root 桶凭据的已知如实报告；缓解路径为 CAM 前缀子凭据，本轮不强制）。
- 验证命令链：`GET /healthz` → `POST /auth/key`（管理员 Key 换会话）→ `POST /api/v1/spaces`（`connection_id:"platform"`）→ `POST /api/v1/uploads` + `PUT /api/v1/operations/{id}/parts/0` + `POST .../complete`（100 KiB 随机负载）→ `GET /api/v1/files/{id}/content` 下载 sha256 比对 **MATCH** → rclone 核对 COS 键形状为 `drivebridge/<instance>/blobs/<sha256>`（内容寻址命名空间）→ `status --url --token-file --json` 远程核对 `reachable:true`、空间用量一致。
- 清理：`POST /{id}/trash` + `DELETE /api/v1/files/{id}`（`X-Confirm-Permanent-Delete: yes`）→ 停服窗口 `blob gc --yes` 两次（回收 102400 + 1024 字节），COS 实例前缀确认为空。
- 重启持久性：经 `temp/lan-preview/start.sh` 完整重启进程后再次 1 KiB 上传/下载比对 **MATCH**，确认封存的 COS 连接随 state 跨重启复用。
- 证据：`temp/integration-test-runs/run-20260917-160020-cos/`（summary/stdout/stderr/env/artifacts，密钥与哈希已脱敏）。
- 操作注意：platform-connect / blob gc 与 serve 互斥 state 锁，需要短暂停服窗口；storage 不可原地重绑到其他桶。
