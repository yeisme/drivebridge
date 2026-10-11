## Context

`Service` 单 issuer、`(issuer, subject)` 用户、`instance.id`（48 hex）随元数据备份恢复；远程 SDK 引用已带实例身份；部署默认关闭、origin 固定。用户已确认定位：部署即租户（个人/家庭=单实例自部署，多租户=多实例）。

## Decisions

- **tenant profile**：`serve --tenant-name/--tenant-origin/--tenant-contact` 首次启动写入 metadata（新表或 KV），之后只读（改配置=停机改启动参数，不做在线租户管理面）；`GET /api/v1/tenant`（登录用户可读）返回 name/origin/instance-id 前缀，供网页页脚与 Agent 提示"当前部署是谁"。
- **隔离陈述**：以 spec 场景固化——跨实例请求引用（实例 ID 不匹配）返回拒绝；跨部署 Agent 访问仅经远程模式 token（绑定目标实例签发）；不提供实例间委托或联邦。
- **升级路径**：多租户=分片部署清单（独立 state 目录、独立 origin/TLS、独立 issuer 或同 issuer 不同 client），运维文档化（deployment.md 增一节），不做单实例多租户改造，不做跨实例迁移工具承诺。
- **命名**：`instance.id` 保持唯一租户标识，不引入第二 ID（避免双标识漂移）；"tenant"仅为语义层词汇。

## Interfaces

`internal/service/model.go`（TenantProfile 记录+迁移）、`http.go`（GET /api/v1/tenant）、`main.go`（启动参数）。文档：`docs/deployment.md` 增"部署即租户与多实例分片"节。

## Verification / Rollback

实现波验证：profile 三参数落库/只读、跨实例引用拒绝场景、/api/v1/tenant 鉴权边界（匿名拒、用户读）。回滚：不传参数=无 profile，接口返回 null profile，零行为变化。

## 去向

实现波：tenant profile + 只读接口 + deployment.md 分片节；根合同侧由 drivebridge-platform-boundary-v2 同步声明。
