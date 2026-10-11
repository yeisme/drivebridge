# Aigora 服务身份接入

DriveBridge 验证 Identity Platform 的 EdDSA 服务凭证，audience 固定 drivebridge，有效期最长五分钟。通过 issuer、subject、tenant 的精确映射找到既有内部用户和租户；不按邮箱关联，不把上游财务角色转换成数据管理员。服务账号必须映射到既有受限 Agent。

映射和切换记录由部署侧命令维护，切换后旧用户 Key、会话和 Agent 不能绕过该租户的统一认证。新接口和字段为增量，旧部署默认关闭。

读取在短期凭证有效且本地权限仍有效时可降级；写入和提权必须在线核验。商业准入应接入业务操作生命周期，不能按 HTTP/MCP 请求次数收费；未接好计量的统一身份写入必须拒绝，不能在认证上线时默认放行免费写入。

领域审批、平台/租户/团队只读管理和凭据排除边界保持原义。身份接入、商业准入、Web/CLI/MCP 体验及迁移验证分别验收，未完成阶段不标记上线。

## 已验证入口

服务使用固定 issuer/JWKS/Identity backend 验证五分钟 EdDSA context，并核对当前 session、成员与来源 Key。`identity-link` 仅按明确的 issuer、subject、身份组织、本地用户与租户建立映射；不能按邮箱合并。切换前检查本租户所有活跃成员已映射；切换后拒绝旧凭据，不通过清除单个映射恢复旧入口。

SDK `NewAigora` 把长期 Key 只送至 Aigora，线程安全缓存短期服务凭证。请求失败时不会回退到长期 Key；中心不可用仅保留未过期凭证覆盖的 GET/HEAD，由后端进一步限制为允许的元数据读取。SDK HTTP 客户端不跟随重定向。MCP 适配器支持 `aigora_url`，客户端不需要安装 CLI；配置在适配器宿主完成。

现有领域管理读取与修改授权保持原义。统一 Key 的跨租户管理上下文与收费读取尚未接通，显式拒绝；托管上传已在业务操作边界接通 service-billing 准入（预留、上报、重启对账），未计量或未配置 reporter 凭据的其余写入仍以 `service_admission_required` fail-closed，不授予管理角色或静默绕过计费。后端 service、SDK、命令与配置回归（包含 race）通过，纯 Go 构建和 vet 通过。
